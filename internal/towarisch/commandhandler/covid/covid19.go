package covid

import (
	"context"
	"fmt"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ilyalavrinov/tgbots/internal/towarisch/commandhandler/yandexnews"
	"github.com/ilyalavrinov/tgbots/pkg/tgbotbase"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

func atoi(s string) int {
	res, err := strconv.Atoi(s)
	if err != nil {
		log.WithFields(log.Fields{"err": err, "str": s}).Error("Could not convert in atoi")
	}
	return res
}

type covid19Handler struct {
	tgbotbase.BaseHandler
	props tgbotbase.PropertyStorage
	cron  tgbotbase.Cron

	updates chan history
	toSend  chan tgbotbase.ChatID
}

var _ tgbotbase.BackgroundMessageHandler = &covid19Handler{}

func NewCovid19Handler(cron tgbotbase.Cron,
	props tgbotbase.PropertyStorage) tgbotbase.BackgroundMessageHandler {
	h := &covid19Handler{
		props: props,
		cron:  cron,

		updates: make(chan history, 0),
		toSend:  make(chan tgbotbase.ChatID, 0),
	}

	h.cron.AddJob(time.Now(), &covidUpdateJob{updates: h.updates})
	return h
}

func (h *covid19Handler) Init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- tgbotbase.ServiceMsg) {
	h.OutMsgCh = outMsgCh
}

func (h *covid19Handler) Run() {
	chatsToNotify := make([]tgbotbase.ChatID, 0)
	props, _ := h.props.GetEveryHavingProperty(context.TODO(), "covid19Time")
	for _, prop := range props {
		if (prop.User != 0) && (tgbotbase.ChatID(prop.User) != prop.Chat) {
			log.Printf("COVID-19: Skipping special setting for user %d in chat %d", prop.User, prop.Chat)
			continue
		}
		chatsToNotify = append(chatsToNotify, prop.Chat)
	}

	countriesOfInterestL10N := map[string]string{
		"World":          "🌎В мире",
		locationRussia:   "🇷🇺Россия",
		"United States":  "🇺🇸США",
		"Italy":          "🍕Италия",
		"China":          "🇨🇳Китай",
		locationRussiaNN: "🦌НижОбла"}
	countriesOfInterest := []string{"World", locationRussia, locationRussiaNN, "United States"}

	go func() {
		for {
			updatedHistory := <-h.updates

			text := fmt.Sprintf("Обновление \\#covid19")
			for _, name := range countriesOfInterest {
				localName := name
				if l10n, found := countriesOfInterestL10N[name]; found {
					localName = l10n
				}
				d, err := updatedHistory.getDay(context.TODO(), name, time.Now())
				if err != nil {
					log.WithFields(log.Fields{"err": err, "location": name}).Error("Failed to get historical data")
					continue
				}
				text = fmt.Sprintf("%s\n***%s***: 🌡 %d \\(\\+%d\\) \\| 💀 %d \\(\\+%d\\)",
					text, localName, d.sickTotal, d.sickInc, d.deadTotal, d.deadInc)
			}
			text = fmt.Sprintf("%s\n[карта](https://gisanddata.maps.arcgis.com/apps/opsdashboard/index.html#/bda7594740fd40299423467b48e9ecf6) \\+ [графики](https://ourworldindata.org/coronavirus#growth-country-by-country-view)", text)
			if news, err := yandexnews.LoadYaNews(yandexnews.YaNewsCovid19); err == nil && len(news) > 0 {
				text = fmt.Sprintf("%s\n\nПоследние новости:", text)
				for _, n := range news {
					text = fmt.Sprintf("%s\n%s", text, n.ToMarkdown())
				}
			}
			for _, chatID := range chatsToNotify {
				msg := tgbotapi.NewMessage(int64(chatID), text)
				msg.ParseMode = "MarkdownV2"
				msg.DisableWebPagePreview = true
				h.OutMsgCh <- msg
			}
		}
	}()
}

func (h *covid19Handler) Name() string {
	return "coronavirus stats at morning"
}
