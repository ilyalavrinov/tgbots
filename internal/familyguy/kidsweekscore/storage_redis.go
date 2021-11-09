package kidsweekscore

import (
	"context"
	"errors"
	"fmt"
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

func key(childName string, timestamp time.Time) string {
	return fmt.Sprintf("kidscore:%s:%s", childName, timestamp.Format(layout))
}

func (s *redisStorage) add(ctx context.Context, childName string, timestamp time.Time, val mark) error {
	return s.client.Set(ctx, key(childName, timestamp), val, ttl).Err()
}

func (s *redisStorage) get(ctx context.Context, childName string, t1, t2 time.Time) ([]mark, error) {
	keys, err := s.client.Keys(ctx, fmt.Sprintf("kidscore:%s:*", childName)).Result()
	if err != nil {
		return nil, err
	}

	result := make([]mark, 0, len(keys))
	for _, k := range keys {
		parts := strings.Split(k, ":")
		if len(parts) != 3 {
			return nil, errors.New(fmt.Sprintf("Key %q cannot be correctly split", k))
		}

		t, err := time.Parse(layout, parts[2])
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

		result = append(result, mark(val))
	}

	return result, nil
}
