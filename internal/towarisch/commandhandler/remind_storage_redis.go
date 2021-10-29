package cmd

import "fmt"
import "time"
import "strings"
import "strconv"
import "errors"
import "log"

import "github.com/go-redis/redis"
import "github.com/admirallarimda/tgbotbase"

var remindStart time.Time = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

type RedisReminderStorage struct {
	client *redis.Client
}

func NewRedisReminderStorage(pool tgbotbase.RedisPool) ReminderStorage {
	s := RedisReminderStorage{client: pool.GetConnByName("reminder")}
	return &s
}

func reminderKey(r Reminder) string {
	tdiff := r.t.Sub(remindStart)
	return fmt.Sprintf("reminder:%d:%d:%d", int(tdiff.Seconds()), r.chat, r.replyTo)
}

func keyToReminder(key string) (*Reminder, error) {
	splits := strings.Split(key, ":")
	if len(splits) != 4 {
		return nil, errors.New(fmt.Sprintf("Reminder key '%s' does not follow the expected format", key))
	}

	tdiffStr := splits[1]
	tdiff, err := strconv.Atoi(tdiffStr)
	if err != nil {
		return nil, err
	}
	t := remindStart.Add(time.Duration(tdiff) * time.Second)

	chatStr := splits[2]
	chat, err := strconv.Atoi(chatStr)
	if err != nil {
		return nil, err
	}

	replyToStr := splits[3]
	replyTo, err := strconv.Atoi(replyToStr)
	if err != nil {
		return nil, err
	}
	return &Reminder{
		t:       t,
		chat:    tgbotbase.ChatID(chat),
		replyTo: replyTo}, nil
}

func (s *RedisReminderStorage) AddReminder(r Reminder) {
	s.client.Set(reminderKey(r), 0, r.t.Add(24*time.Hour).Sub(time.Now()))
}

func (s *RedisReminderStorage) RemoveReminder(r Reminder) {
	s.client.Del(reminderKey(r))
}

func (s *RedisReminderStorage) LoadAll() []Reminder {
	keys, err := tgbotbase.GetAllKeys(s.client, "reminder:*")
	if err != nil {
		log.Printf("redisReminder: could not load stored reminders due to error: %s", err)
		return nil
	}
	log.Printf("redisReminder: loaded %d keys", len(keys))
	reminders := make([]Reminder, 0, len(keys))
	for _, k := range keys {
		r, err := keyToReminder(k)
		if err != nil {
			log.Printf("redisReminder: could not convert reminder key '%s' due to error: %s", k, err)
		} else {
			log.Printf("redisReminder: new reminder: %+v", *r)
			reminders = append(reminders, *r)
		}
	}
	return reminders
}
