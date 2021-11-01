package tgbotbase

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
)

type RedisPropertyStorage struct {
	client *redis.Client
}

func NewRedisPropertyStorage(pool RedisPool) *RedisPropertyStorage {
	r := &RedisPropertyStorage{client: pool.GetConnByName("property")}
	return r
}

func redisPropertyKey(name string, user UserID, chat ChatID) string {
	if strings.Contains(name, ":") {
		panic(fmt.Sprintf("Property key %q contains forbidden symbol %q", name, ":"))
	}
	return fmt.Sprintf("tg:property:%s:%d:%d", name, user, chat)
}

func (r *RedisPropertyStorage) SetPropertyForUserInChat(ctx context.Context, name string, user UserID, chat ChatID, value interface{}) error {
	log.Printf("Setting property '%s' for user %d chat %d with value: %v", name, user, chat, value)
	key := redisPropertyKey(name, user, chat)
	return r.client.Set(ctx, key, value, 0).Err()
}

func (r *RedisPropertyStorage) SetPropertyForUser(ctx context.Context, name string, user UserID, value interface{}) error {
	log.Printf("Setting property '%s' for user %d with value: %v", name, user, value)
	return r.SetPropertyForUserInChat(ctx, name, user, ChatID(user), value)
}

func (r *RedisPropertyStorage) SetPropertyForChat(ctx context.Context, name string, chat ChatID, value interface{}) error {
	log.Printf("Setting property '%s' for chat %d with value: %v", name, chat, value)
	return r.SetPropertyForUserInChat(ctx, name, 0, chat, value)
}

func (r *RedisPropertyStorage) GetProperty(ctx context.Context, name string, user UserID, chat ChatID) (string, error) {
	log.Printf("Getting property '%s' for user %d chat %d", name, user, chat)

	// checking specific property value for this user in this chat
	res := r.client.Get(ctx, redisPropertyKey(name, user, chat))
	err := res.Err()
	if err != nil {
		if err == redis.Nil {
			log.Printf("No property '%s' for user %d chat %d, checking next", name, user, chat)
		} else {
			return "", err
		}
	} else {
		return res.Val(), nil
	}

	// checking user-defined property (for any chat, set via direct msg)
	res = r.client.Get(ctx, redisPropertyKey(name, user, ChatID(user)))
	err = res.Err()
	if err != nil {
		if err == redis.Nil {
			log.Printf("No property '%s' for user %d, checking next", name, user)
		} else {
			return "", err
		}
	} else {
		return res.Val(), nil
	}

	// checking chat-defined property (default property for this chat)
	res = r.client.Get(ctx, redisPropertyKey(name, 0, chat))
	err = res.Err()
	if err != nil {
		if err == redis.Nil {
			log.Printf("No property '%s' for chat %d", name, chat)
		} else {
			return "", err
		}
	} else {
		return res.Val(), nil
	}

	log.Printf("No property '%s' for user %d chat %d, returning null", name, user, chat)
	return "", nil
}

func (r *RedisPropertyStorage) GetEveryHavingProperty(ctx context.Context, name string) ([]PropertyValue, error) {
	log.Printf("Getting property '%s' for every chat", name)
	pattern := fmt.Sprintf("tg:property:%s:*:*", name)
	keys, err := GetAllKeys(ctx, r.client, pattern)
	if err != nil {
		return nil, err
	}
	props := make([]PropertyValue, 0, len(keys))
	for _, k := range keys {
		value, err := r.client.Get(ctx, k).Result()
		if err != nil {
			log.Printf("Property by key '%s' could not be retrieved due to error: %s", k, err)
			continue
		}

		parts := strings.Split(k, ":")
		if len(parts) != 5 {
			log.Printf("Key '%s' has unexpected number of parts", k)
			continue
		}
		userStr := parts[3]
		userID, err := strconv.Atoi(userStr)
		if err != nil {
			log.Printf("Could not convert user '%s' to integer due to error: %s", userStr, err)
			continue
		}

		chatStr := parts[4]
		chatID, err := strconv.Atoi(chatStr)
		if err != nil {
			log.Printf("Could not convert chat '%s' to integer due to error: %s", chatStr, err)
			continue
		}

		props = append(props, PropertyValue{
			User:  UserID(userID),
			Chat:  ChatID(chatID),
			Value: value})
	}

	return props, nil
}

var _ PropertyStorage = &RedisPropertyStorage{}
