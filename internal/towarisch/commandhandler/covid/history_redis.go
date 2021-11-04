package covid

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/ilyalavrinov/tgbots/pkg/tgbotbase"
)

type redisHistory struct {
	client *redis.Client
}

var _ History = &redisHistory{}

func NewRedisHistory(pool tgbotbase.RedisPool) *redisHistory {
	return &redisHistory{
		client: pool.GetConnByName("covid"),
	}
}

var errTooOld = errors.New("incoming data too old to put into cache")

const (
	ageLimit = 7 * 24 * time.Hour
	ttl      = 7 * 24 * time.Hour
)

func key(location string, day time.Time) string {
	return fmt.Sprintf("covid:history:%s:%s", strings.ReplaceAll(location, " ", ""), day.Format("20060102"))
}

func (r *redisHistory) add(ctx context.Context, location string, day time.Time, totalSick, totalDead int) error {
	if time.Now().Sub(day) > 7*24*time.Hour {
		return errTooOld
	}
	_, err := r.client.HSet(ctx, key(location, day), map[string]interface{}{"sick": totalSick, "dead": totalDead}).Result()
	if err != nil {
		return err
	}
	return r.client.Expire(ctx, key(location, day), ttl).Err()
}

func (r *redisHistory) addIfNotExist(ctx context.Context, location string, day time.Time, totalSick, totalDead int) (bool, error) {
	exist, err := r.client.Exists(ctx, key(location, day)).Result()
	if err != nil {
		return false, err
	}
	if exist > 0 {
		return false, nil
	}

	err = r.add(ctx, location, day, totalSick, totalDead)
	if err != nil {
		if errors.Is(err, errTooOld) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func convDay(res map[string]string) (dayData, error) {
	sick, err := strconv.Atoi(res["sick"])
	if err != nil {
		return dayData{}, err
	}
	dead, err := strconv.Atoi(res["dead"])
	if err != nil {
		return dayData{}, err
	}

	return dayData{
		sickTotal: sick,
		deadTotal: dead,
	}, nil
}

func (r *redisHistory) doGetDay(ctx context.Context, location string, day time.Time) (dayData, error) {
	res, err := r.client.HGetAll(ctx, key(location, day)).Result()
	if err != nil {
		return dayData{}, err
	}
	targetDay, err := convDay(res)
	if err != nil {
		return dayData{}, err
	}
	return targetDay, nil
}

func fillIncrease(day0, day1, day2 dayData) dayData {
	day1.sickInc = day1.sickTotal - day0.sickTotal
	day1.deadInc = day1.deadTotal - day0.deadTotal

	result := dayData{
		sickTotal:     day2.sickTotal,
		sickInc:       day2.sickTotal - day1.sickTotal,
		sickIncGrowth: (day2.sickTotal - day1.sickTotal) - day1.sickInc,

		deadTotal:     day2.deadTotal,
		deadInc:       day2.deadTotal - day1.deadTotal,
		deadIncGrowth: (day2.deadTotal - day1.deadTotal) - day1.deadInc,
	}

	return result
}

func (r *redisHistory) getDay(ctx context.Context, location string, day time.Time) (dayData, error) {
	targetDay, err := r.doGetDay(ctx, location, day)
	if err != nil {
		return dayData{}, err
	}
	prevDay, err := r.doGetDay(ctx, location, day.Add(-24*time.Hour))
	if err != nil {
		return dayData{}, err
	}
	prevprevDay, err := r.doGetDay(ctx, location, day.Add(-24*2*time.Hour))
	if err != nil {
		return dayData{}, err
	}

	return fillIncrease(prevprevDay, prevDay, targetDay), nil
}
