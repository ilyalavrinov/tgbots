package covid

import (
	"context"
	"time"
)

type dayData struct {
	sickTotal     int
	sickInc       int
	sickIncGrowth int

	deadTotal     int
	deadInc       int
	deadIncGrowth int
}

type history interface {
	add(ctx context.Context, location string, day time.Time, totalSick, totalDead int) error
	addIfNotExist(ctx context.Context, location string, day time.Time, totalSick, totalDead int) (added bool, err error)
	getDay(ctx context.Context, location string, day time.Time) (dayData, error)
}
