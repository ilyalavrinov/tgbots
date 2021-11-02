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

const (
	nnID = "52region"
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

	updates chan covidData
	toSend  chan tgbotbase.ChatID
}

var _ tgbotbase.BackgroundMessageHandler = &covid19Handler{}

func NewCovid19Handler(cron tgbotbase.Cron,
	props tgbotbase.PropertyStorage) tgbotbase.BackgroundMessageHandler {
	h := &covid19Handler{
		props: props,
		cron:  cron,

		updates: make(chan covidData, 0),
		toSend:  make(chan tgbotbase.ChatID, 0),
	}
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
		"World":         "🌎В мире",
		"Russia":        "🇷🇺Россия",
		"United States": "🇺🇸США",
		"Italy":         "🍕Италия",
		"China":         "🇨🇳Китай",
		nnID:            "🦌НижОбла"}
	countriesOfInterest := []string{"World", "Russia", nnID, "United States"}
	prevLastCasesS, _ := h.props.GetProperty(context.TODO(), "covidLastCasesRussia", tgbotbase.UserID(0), tgbotbase.ChatID(0))
	prevLastCases, err := strconv.Atoi(prevLastCasesS)
	if err != nil {
		prevLastCases = 0
	}

	go func() {
		data := covidData{}
		for {
			select {
			case data = <-h.updates:
				lastCases := data.countryLatest["Russia"].totalCases
				log.WithFields(log.Fields{"prev": prevLastCases, "new": lastCases}).Debug("New update received")
				if lastCases <= prevLastCases {
					continue
				}
				prevLastCases = lastCases
				h.props.SetPropertyForUserInChat(context.TODO(), "covidLastCasesRussia", tgbotbase.UserID(0), tgbotbase.ChatID(0), strconv.Itoa(lastCases))

				text := fmt.Sprintf("Обновление \\#covid19")
				for _, name := range countriesOfInterest {
					localName := name
					if l10n, found := countriesOfInterestL10N[name]; found {
						localName = l10n
					}
					if cases, found := data.countryLatest[name]; found {
						text = fmt.Sprintf("%s\n***%s***: 🌡 %d \\(\\+%d\\) \\| 💀 %d \\(\\+%d\\)",
							text, localName, cases.totalCases, cases.newCases, cases.totalDeaths, cases.newDeaths)
					}
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
		}
	}()

	h.cron.AddJob(time.Now(), &covidUpdateJob{updates: h.updates})
}

func (h *covid19Handler) Name() string {
	return "coronavirus stats at morning"
}

type covidJob struct {
	chatID tgbotbase.ChatID
	ch     chan<- tgbotbase.ChatID
}

var _ tgbotbase.CronJob = &covidJob{}

const (
	colDate        = 0
	colCountry     = 1
	colNewCases    = 2
	colNewDeaths   = 3
	colTotalCases  = 4
	colTotalDeaths = 5
)

func (job *covidJob) Do(scheduledWhen time.Time, cron tgbotbase.Cron) {
	defer cron.AddJob(scheduledWhen.Add(24*time.Hour), job)

	job.ch <- job.chatID
}
