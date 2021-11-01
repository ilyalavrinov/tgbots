package tgbotbase

import (
	"context"
	"log"
	"strings"

	"github.com/go-redis/redis/v8"
)

type RedisPool interface {
	GetConnByID(dbID int) *redis.Client
	GetConnByName(dbName string) *redis.Client
}

type RedisConfig struct {
	Server string
	Pass   string
}

type RedisPoolImpl struct {
	cfg RedisConfig
	db  map[string]int
}

func NewRedisPool(ctx context.Context, cfg RedisConfig) RedisPool {
	impl := RedisPoolImpl{cfg: cfg,
		db: make(map[string]int, 10)}

	// loading dictionary for db discovery
	opts := redis.Options{Addr: cfg.Server,
		Password: cfg.Pass,
		DB:       0}
	conn := redis.NewClient(&opts)
	if conn == nil {
		log.Panicf("Could not connect to Redis using configuration: %+v", cfg)
	}

	keys, err := GetAllKeys(ctx, conn, "db:*")
	if err == nil {
		for _, key := range keys {
			dbID, err := conn.Get(ctx, key).Int64()
			if err != nil {
				log.Printf("Could not get db ID for key '%s' due to error: %s; skipping", key, err)
				continue
			}
			dbname := strings.Split(key, ":")[1]
			log.Printf("Redis DB '%s' is located at DB id %d", dbname, dbID)
			impl.db[dbname] = int(dbID)
		}
	}

	return &impl
}

func (pool *RedisPoolImpl) GetConnByID(dbID int) *redis.Client {
	opts := redis.Options{Addr: pool.cfg.Server,
		Password: pool.cfg.Pass,
		DB:       dbID}
	return redis.NewClient(&opts)
}

func (pool *RedisPoolImpl) GetConnByName(dbName string) *redis.Client {
	dbID, found := pool.db[dbName]
	if !found {
		log.Fatalf("DB named '%s' not known to the pool", dbName)
		return nil
	}
	return pool.GetConnByID(dbID)
}

// GetAllKeys returns unique slice of keys matching the pattern
func GetAllKeys(ctx context.Context, conn *redis.Client, matchPattern string) ([]string, error) {
	log.Printf("Starting scanning for match '%s'", matchPattern)
	result := make([]string, 0)
	var cursor uint64 = 0
	for {
		keys, newcursor, err := conn.Scan(ctx, cursor, matchPattern, 100).Result()
		if err != nil {
			log.Printf("Error happened while scanning with match pattern '%s', error: %s", matchPattern, err)
			return nil, err
		}
		cursor = newcursor
		result = append(result, keys...)
		if cursor == 0 {
			log.Printf("Scanning for '%s' has finished, result contains %d elements", matchPattern, len(result))
			break
		}
	}
	log.Printf("Scanner '%s' returned %d keys", matchPattern, len(result))
	return uniqueStringSlice(result), nil
}

func uniqueStringSlice(s []string) []string {
	result := make([]string, 0, len(s))
	seen := make(map[string]bool, len(s))
	for _, elem := range s {
		if _, found := seen[elem]; found {
			continue
		}
		result = append(result, elem)
		seen[elem] = true
	}
	return result
}
