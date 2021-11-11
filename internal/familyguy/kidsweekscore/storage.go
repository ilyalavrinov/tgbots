package kidsweekscore

import (
	"context"
	"time"
)

type mark string

const (
	Good mark = "+"
	Bad  mark = "-"
)

type Storage interface {
	add(ctx context.Context, chatId int64, childName string, timestamp time.Time, val mark) error
	get(ctx context.Context, chatId int64, childName string, t1, t2 time.Time) ([]mark, error)
	loadSettings(ctx context.Context, chatId int64) (parents []string, kids map[string][]string, err error)
}
