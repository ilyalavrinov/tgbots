package cmd

import (
	"log"
	"time"

	"github.com/admirallarimda/tgbotbase"
	"github.com/go-redis/redis"
	"gopkg.in/telegram-bot-api.v4"
)

type weatherMorningHandler struct {
	tgbotbase.BaseHandler
	props tgbotbase.PropertyStorage
	conn  *redis.Client
	cron  tgbotbase.Cron
	token string
}

var _ tgbotbase.BackgroundMessageHandler = &weatherMorningHandler{}

func NewWeatherMorningHandler(cron tgbotbase.Cron,
	props tgbotbase.PropertyStorage,
	pool tgbotbase.RedisPool,
	token string) tgbotbase.BackgroundMessageHandler {
	h := &weatherMorningHandler{
		props: props,
		conn:  pool.GetConnByName("openweathermap"),
		cron:  cron,
		token: token}
	return h
}

func (h *weatherMorningHandler) Init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- tgbotbase.ServiceMsg) {
	h.OutMsgCh = outMsgCh
}

func (h *weatherMorningHandler) Run() {
	// TODO: same as for kitties. Write common func
	now := time.Now()
	props, _ := h.props.GetEveryHavingProperty("weatherTime")
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

		cityID, err := getCityIDFromProperty(h.props, h.conn, prop.User, prop.Chat)
		if err != nil {
			log.Printf("Could not get city ID from property for user '%d' city '%d' due to error: %s", prop.User, prop.Chat, err)
			continue
		}

		when := tgbotbase.CalcNextTimeFromMidnight(now, dur)
		job := weatherJob{
			cityID: cityID,
			chatID: prop.Chat,
			token:  h.token}
		job.OutMsgCh = h.OutMsgCh
		h.cron.AddJob(when, &job)
	}
}

func (h *weatherMorningHandler) Name() string {
	return "weather at morning"
}

type weatherJob struct {
	tgbotbase.BaseHandler
	cityID int64
	chatID tgbotbase.ChatID
	token  string
}

var _ tgbotbase.CronJob = &weatherJob{}

func (job *weatherJob) Do(scheduledWhen time.Time, cron tgbotbase.Cron) {
	defer cron.AddJob(scheduledWhen.Add(24*time.Hour), job)

	if msg, err := getForecast(job.token, job.cityID, time.Now()); err == nil {
		job.OutMsgCh <- tgbotapi.NewMessage(int64(job.chatID), msg)
	}
}
