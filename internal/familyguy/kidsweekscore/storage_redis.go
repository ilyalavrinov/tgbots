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

func key(chatId int64, childName string, timestamp time.Time) string {
	return fmt.Sprintf("kidscore:%d:kid:%s:%s", chatId, childName, timestamp.Format(layout))
}

func (s *redisStorage) add(ctx context.Context, chatId int64, childName string, timestamp time.Time, val mark) error {
	return s.client.Set(ctx, key(chatId, childName, timestamp), val, ttl).Err()
}

func (s *redisStorage) get(ctx context.Context, chatId int64, childName string, t1, t2 time.Time) ([]mark, error) {
	keys, err := s.client.Keys(ctx, fmt.Sprintf("kidscore:%d:kid:%s:*", chatId, childName)).Result()
	if err != nil {
		return nil, err
	}

	result := make([]mark, 0, len(keys))
	for _, k := range keys {
		parts := strings.Split(k, ":")
		if len(parts) != 4 {
			return nil, errors.New(fmt.Sprintf("Key %q cannot be correctly split", k))
		}

		t, err := time.Parse(layout, parts[3])
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

func (s *redisStorage) loadSettings(ctx context.Context, chatId int64) ([]string, map[string][]string, error) {
	parents, err := s.client.LRange(ctx, fmt.Sprintf("kidscore:%d:parents", chatId), 0, -1).Result()
	if err != nil {
		return nil, nil, err
	}

	keys, err := s.client.Keys(ctx, fmt.Sprintf("kidscore:%d:kidAlias:*", chatId)).Result()
	if err != nil {
		return nil, nil, err
	}
	kids := make(map[string][]string, len(keys))
	for _, k := range keys {
		parts := strings.Split(k, ":")
		if len(parts) != 4 {
			return nil, nil, errors.New(fmt.Sprintf("Key %q cannot be correctly split", k))
		}
		kidName := parts[1]
		aliases, err := s.client.LRange(ctx, k, 0, -1).Result()
		if err != nil {
			return nil, nil, err
		}
		kids[kidName] = aliases
	}

	return parents, kids, nil
}
