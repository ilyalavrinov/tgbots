package kidsweekscore

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ilyalavrinov/tgbots/pkg/tgbotbase"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

type kidScoreResult struct {
	tgbotbase.BaseHandler

	storage Storage
	cron    tgbotbase.Cron
	props   tgbotbase.PropertyStorage
}

var _ tgbotbase.BackgroundMessageHandler = &kidScoreResult{}

func NewKidScoreResult(storage Storage, cron tgbotbase.Cron, props tgbotbase.PropertyStorage) tgbotbase.BackgroundMessageHandler {
	return &kidScoreResult{
		storage: storage,
		cron:    cron,
		props:   props,
	}
}

func (h *kidScoreResult) Init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- tgbotbase.ServiceMsg) {
	h.OutMsgCh = outMsgCh
}

func (h *kidScoreResult) Name() string {
	return "kid weekly score"
}

func (h *kidScoreResult) Run() {
	props, _ := h.props.GetEveryHavingProperty(context.TODO(), "kidsScoreResultTime")
	for _, prop := range props {
		if (prop.User != 0) && (tgbotbase.ChatID(prop.User) != prop.Chat) {
			log.Printf("Morning weather: Skipping special setting for user %d in chat %d", prop.User, prop.Chat)
			continue
		}
		dur, err := time.ParseDuration(prop.Value)
		if err != nil {
			log.Printf("Could not parse duration %s for chat %d due to error: %s", prop.Value, prop.Chat, err)
			continue
		}

		dayMult := 0
		d := time.Now().Weekday()
		if d != time.Sunday {
			dayMult = 7 - int(d)
		}
		triggerDay := time.Now().Add(time.Duration(dayMult) * 24 * time.Hour)
		when := tgbotbase.CalcNextTimeFromMidnight(triggerDay, dur)
		job := kidScoreResultJob{
			chatID:  prop.Chat,
			storage: h.storage,
		}
		job.OutMsgCh = h.OutMsgCh
		h.cron.AddJob(when, &job)
	}
}

type kidScoreResultJob struct {
	tgbotbase.BaseHandler
	chatID  tgbotbase.ChatID
	storage Storage
}

var _ tgbotbase.CronJob = &kidScoreResultJob{}

func (job *kidScoreResultJob) Do(scheduledWhen time.Time, cron tgbotbase.Cron) {
	defer cron.AddJob(scheduledWhen.Add(7*24*time.Hour), job)

	ctx := context.TODO()
	_, kids, err := job.storage.loadSettings(ctx, int64(job.chatID))
	if err != nil {
		log.WithFields(log.Fields{"err": err, "chat": job.chatID}).Error("Could not load settings")
		return
	}
	msg := "Недельный результаты:"
	for kid := range kids {
		t2 := time.Now()
		t1 := t2.Add(-7 * 24 * time.Hour)

		marks, err := job.storage.get(ctx, int64(job.chatID), kid, t1, t2)
		if err != nil {
			log.WithFields(log.Fields{"err": err, "chat": job.chatID, "kid": kid}).Error("Could not load marks")
			return
		}
		positives := 0
		negatives := 0
		for _, mark := range marks {
			switch mark {
			case Good:
				positives++
			case Bad:
				negatives++
			}
		}
		total := positives + negatives
		msg = fmt.Sprintf("%s\n%s: %d :) ; %d :( ; всего: %d", msg, kid, positives, negatives, total)
	}
	job.OutMsgCh <- tgbotapi.NewMessage(int64(job.chatID), msg)
}
