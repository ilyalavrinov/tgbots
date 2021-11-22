package kidsweekscore

import (
	"context"
	"time"
)

const (
	Unknown string = "?"
	Good    string = "+"
	Bad     string = "-"
)

type settings struct {
	parents       []string
	kidsAliases   map[string][]string
	kidsBirthdays map[string]time.Time
	baseRate      int
}

type Storage interface {
	add(ctx context.Context, chatId int64, childName string, timestamp time.Time, val string) error
	get(ctx context.Context, chatId int64, childName string, t1, t2 time.Time) ([]string, error)
	loadSettings(ctx context.Context, chatId int64) (settings, error)
}
