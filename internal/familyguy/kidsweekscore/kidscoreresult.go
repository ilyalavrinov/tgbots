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

		when := tgbotbase.CalcNextTriggerDay(time.Now(), time.Sunday, dur)
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
	settings, err := job.storage.loadSettings(ctx, int64(job.chatID))
	if err != nil {
		log.WithFields(log.Fields{"err": err, "chat": job.chatID}).Error("Could not load settings")
		return
	}
	msg := "Недельные результаты:"
	for kid := range settings.kidsAliases {
		positives, negatives, err := scoresThisWeek(ctx, job.storage, int64(job.chatID), kid)
		if err != nil {
			log.WithFields(log.Fields{"err": err, "chat": job.chatID, "kid": kid}).Error("Could not load marks")
			return
		}
		total := positives + negatives
		age := time.Now().Sub(settings.kidsBirthdays[kid]) % (365 * 24 * time.Hour)
		totalMoney := float32(settings.baseRate * int(age))
		moneyToKid := int(totalMoney * float32(positives) / float32(total))
		log.WithFields(log.Fields{"kid": kid, "+": positives, "-": negatives, "total": total, "age": age, "totalMoney": totalMoney, "moneyToKid": moneyToKid}).Debug("week money calculation")
		msg = fmt.Sprintf("%s\n\n%s: '+' %d; '-' %d", msg, kid, positives, negatives)
		msg = fmt.Sprintf("%s\n%d в копилку; %d на приставку", msg, moneyToKid, int(totalMoney)-moneyToKid)
	}
	job.OutMsgCh <- tgbotapi.NewMessage(int64(job.chatID), msg)
}

func scoresThisWeek(ctx context.Context, storage Storage, chatId int64, kid string) (int, int, error) {
	t2 := time.Now()
	dayMult := t2.Weekday() - time.Monday
	if dayMult < 0 {
		dayMult += 7
	}
	t1 := t2.Add(-time.Duration(dayMult) * 24 * time.Hour).Truncate(24 * time.Hour)
	log.WithFields(log.Fields{"t1": t1.Format(time.RFC3339), "t2": t2.Format(time.RFC3339), "kid": kid, "chatID": chatId}).Debug("Loading marks")
	marks, err := storage.get(ctx, chatId, kid, t1, t2)
	if err != nil {
		return 0, 0, err
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
	return positives, negatives, nil
}
