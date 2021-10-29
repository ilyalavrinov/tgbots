package cmd

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/admirallarimda/tgbotbase"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

type newsNNHandler struct {
	tgbotbase.BaseHandler
	properties tgbotbase.PropertyStorage
	cron       tgbotbase.Cron
}

func NewNewsNNHandler(cron tgbotbase.Cron, properties tgbotbase.PropertyStorage) tgbotbase.BackgroundMessageHandler {
	handler := newsNNHandler{
		properties: properties,
		cron:       cron}
	return &handler
}

func (h *newsNNHandler) Init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- tgbotbase.ServiceMsg) {
	h.OutMsgCh = outMsgCh
}

func (h *newsNNHandler) Name() string {
	return "NN news"
}

func (h *newsNNHandler) Run() {
	now := time.Now()
	props, _ := h.properties.GetEveryHavingProperty("nnNewsTime")
	for _, prop := range props {
		if (prop.User != 0) && (tgbotbase.ChatID(prop.User) != prop.Chat) {
			log.Printf("NN News: Skipping special setting for user %d in chat %d", prop.User, prop.Chat)
			continue
		}
		dur, err := time.ParseDuration(prop.Value)
		if err != nil {
			log.Printf("Could not parse duration %s for chat %d due to error: %s", prop.Value, prop.Chat, err)
			continue
		}
		when := tgbotbase.CalcNextTimeFromMidnight(now, dur)
		job := newsNNJob{chatID: prop.Chat}
		job.OutMsgCh = h.OutMsgCh
		h.cron.AddJob(when, &job)
	}
}

type newsNNJob struct {
	tgbotbase.BaseHandler
	chatID tgbotbase.ChatID
}

func (job *newsNNJob) Do(scheduledWhen time.Time, cron tgbotbase.Cron) {
	defer cron.AddJob(scheduledWhen.Add(24*time.Hour), job)

	news, err := loadYaNews(YaNewsNN)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("error loading NN news")
		return
	}

	if len(news) == 0 {
		log.Error("no news loaded")
		return
	}

	text := "Нижегородские вести:"
	for _, n := range news {
		text = fmt.Sprintf("%s\n%s", text, n.toMarkdown())
	}

	msg := tgbotapi.NewMessage(int64(job.chatID), text)
	msg.ParseMode = "MarkdownV2"
	msg.DisableWebPagePreview = true
	job.OutMsgCh <- msg
}
