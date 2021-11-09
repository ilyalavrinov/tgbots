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
	add(ctx context.Context, childName string, timestamp time.Time, val mark) error
	get(ctx context.Context, childName string, t1, t2 time.Time) ([]mark, error)
}
