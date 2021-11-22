package kidsweekscore

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

type redisStorage struct {
	client *redis.Client
}

var _ Storage = &redisStorage{}

const (
	ttl = 3 * 7 * 24 * time.Hour
)

func NewRedisStorage(pool tgbotbase.RedisPool) *redisStorage {
	return &redisStorage{
		client: pool.GetConnByName("kidsweekscore"),
	}
}

var layout = "20060102T150405.999"
var birthdayLayout = "20060102"

func key(chatId int64, childName string, timestamp time.Time) string {
	return fmt.Sprintf("kidscore:%d:kid:%s:%s", chatId, childName, timestamp.Format(layout))
}

func (s *redisStorage) add(ctx context.Context, chatId int64, childName string, timestamp time.Time, val string) error {
	return s.client.Set(ctx, key(chatId, childName, timestamp), val, ttl).Err()
}

func (s *redisStorage) get(ctx context.Context, chatId int64, childName string, t1, t2 time.Time) ([]string, error) {
	keys, err := s.client.Keys(ctx, fmt.Sprintf("kidscore:%d:kid:%s:*", chatId, childName)).Result()
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(keys))
	for _, k := range keys {
		parts := strings.Split(k, ":")
		if len(parts) != 5 {
			return nil, errors.New(fmt.Sprintf("Key %q cannot be correctly split", k))
		}

		t, err := time.Parse(layout, parts[4])
		if err != nil {
			return nil, err
		}

		if t.Before(t1) || t.After(t2) {
			continue
		}

		val, err := s.client.Get(ctx, k).Result()
		if err != nil {
			return nil, err
		}

		result = append(result, val)
	}

	return result, nil
}

func (s *redisStorage) loadSettings(ctx context.Context, chatId int64) (settings, error) {
	parents, err := s.client.LRange(ctx, fmt.Sprintf("kidscore:%d:parents", chatId), 0, -1).Result()
	if err != nil {
		return settings{}, err
	}

	keys, err := s.client.Keys(ctx, fmt.Sprintf("kidscore:%d:kidAlias:*", chatId)).Result()
	if err != nil {
		return settings{}, err
	}
	kids := make(map[string][]string, len(keys))
	for _, k := range keys {
		parts := strings.Split(k, ":")
		if len(parts) != 4 {
			return settings{}, errors.New(fmt.Sprintf("Key %q cannot be correctly split", k))
		}
		kidName := parts[3]
		aliases, err := s.client.LRange(ctx, k, 0, -1).Result()
		if err != nil {
			return settings{}, err
		}
		kids[kidName] = aliases
		kids[kidName] = append(kids[kidName], kidName)
	}

	kidsBirthdays := make(map[string]time.Time)
	for k := range kids {
		bdayStr, err := s.client.Get(ctx, fmt.Sprintf("kidscore:%d:kidAge:%s", chatId, k)).Result()
		if err != nil {
			return settings{}, err
		}
		bday, err := time.Parse(birthdayLayout, bdayStr)
		if err != nil {
			return settings{}, err
		}
		kidsBirthdays[k] = bday
	}

	rateStr, err := s.client.Get(ctx, fmt.Sprintf("kidscore:%d:baseRate", chatId)).Result()
	if err != nil {
		return settings{}, err
	}
	rate, err := strconv.Atoi(rateStr)
	if err != nil {
		return settings{}, err
	}

	res := settings{
		parents:       parents,
		kidsAliases:   kids,
		kidsBirthdays: kidsBirthdays,
		baseRate:      rate,
	}
	return res, nil
}
