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

type Storage interface {
	add(ctx context.Context, chatId int64, childName string, timestamp time.Time, val string) error
	get(ctx context.Context, chatId int64, childName string, t1, t2 time.Time) ([]string, error)
	loadSettings(ctx context.Context, chatId int64) (parents []string, kids map[string][]string, err error)
}
