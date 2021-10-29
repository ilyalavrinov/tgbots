package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly"
	log "github.com/sirupsen/logrus"

	"github.com/admirallarimda/tgbotbase"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

var markdownToEscape = []string{"\\", "`", "*", "_", "{", "}", "[", "]", "(", ")", "#", "+", "-", ".", "!", "|"}

const (
	nnID = "52region"
)

func escapeMarkdownSpecial(s string) string {
	for _, e := range markdownToEscape {
		s = strings.Replace(s, e, "\\"+e, -1)
	}
	return s
}

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
	props, _ := h.props.GetEveryHavingProperty("covid19Time")
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
	prevLastCasesS, _ := h.props.GetProperty("covidLastCasesRussia", tgbotbase.UserID(0), tgbotbase.ChatID(0))
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
				h.props.SetPropertyForUserInChat("covidLastCasesRussia", tgbotbase.UserID(0), tgbotbase.ChatID(0), strconv.Itoa(lastCases))

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
				if news, err := loadYaNews(YaNewsCovid19); err == nil && len(news) > 0 {
					text = fmt.Sprintf("%s\n\nПоследние новости:", text)
					for _, n := range news {
						text = fmt.Sprintf("%s\n%s", text, n.toMarkdown())
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

var _ tgbotbase.CronJob = &weatherJob{}

const (
	colDate        = 0
	colCountry     = 1
	colNewCases    = 2
	colNewDeaths   = 3
	colTotalCases  = 4
	colTotalDeaths = 5
)

type casesData struct {
	date        time.Time
	newCases    int
	totalCases  int
	newDeaths   int
	totalDeaths int
}

func (job *covidJob) Do(scheduledWhen time.Time, cron tgbotbase.Cron) {
	defer cron.AddJob(scheduledWhen.Add(24*time.Hour), job)

	job.ch <- job.chatID
}

func downloadFile(filepath string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

type covidData struct {
	countryRaw    map[string][]casesData
	countryLatest map[string]casesData
	worldLatest   casesData
}

type covidUpdateJob struct {
	updates chan<- covidData
}

func (j *covidUpdateJob) Do(scheduledWhen time.Time, cron tgbotbase.Cron) {
	defer cron.AddJob(scheduledWhen.Add(30*time.Minute), j)

	log.Debug("Start covid update")
	url := "https://covid.ourworldindata.org/data/ecdc/full_data.csv"
	fpath := path.Join("/tmp", "ilya-tgbot", "covid")
	if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
		log.Printf("Could not create covid directories at %q, err: %s", fpath, err)
		return
	}
	fname := path.Join(fpath, fmt.Sprintf("cases-%s.csv", time.Now().Format("20060102150405")))
	if err := downloadFile(fname, url); err != nil {
		log.Printf("Could not download covid info from %q to %q, err: %s", url, fname, err)
		return
	}

	f, err := os.Open(fname)
	if err != nil {
		log.Printf("Could not open covid info at %q, err: %s", fname, err)
		return
	}

	r := csv.NewReader(f)
	data, err := r.ReadAll()
	if err != nil {
		log.Printf("Could not read covid info at %q, err: %s", fname, err)
		return
	}

	raw := make(map[string][]casesData, 200)
	latest := make(map[string]casesData, 200)
	dates := make(map[string]time.Time, 200)
	for _, line := range data {
		d, _ := time.Parse("2006-01-02", line[colDate])

		newCases, _ := strconv.Atoi(line[colNewCases])
		totalCases, _ := strconv.Atoi(line[colTotalCases])
		newDeaths, _ := strconv.Atoi(line[colNewDeaths])
		totalDeaths, _ := strconv.Atoi(line[colTotalDeaths])
		cinfo := casesData{
			date:        d,
			newCases:    newCases,
			totalCases:  totalCases,
			newDeaths:   newDeaths,
			totalDeaths: totalDeaths,
		}

		// assuming that dates are ordered
		country := line[colCountry]
		raw[country] = append(raw[country], cinfo)

		date, found := dates[country]
		if found && d.Before(date) {
			continue
		}

		dates[country] = d
		latest[country] = cinfo
	}

	russiaData, err := getRussiaData()
	if err == nil {
		for key, val := range russiaData {
			latest[key] = val
		}
	}

	j.updates <- covidData{
		countryRaw:    raw,
		countryLatest: latest,
	}
}

type rusTotalStats struct {
	Sick          string
	sickVal       int
	SickChange    string
	sickChangeVal int
	Died          string
	diedVal       int
	DiedChange    string
	diedChangeVal int
}

func (stat *rusTotalStats) toInt() {
	stat.sickVal = statToInt(stat.Sick)
	stat.sickChangeVal = statToInt(stat.SickChange)
	stat.diedVal = statToInt(stat.Died)
	stat.diedChangeVal = statToInt(stat.DiedChange)
}

func statToInt(s string) int {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "+", "")

	return atoi(s)
}

const (
	nnRegionCode = "RU-NIZ"
)

type regionStats struct {
	Code     string
	Sick     int
	Died     int
	SickIncr int `json:"sick_incr"`
	DiedIncr int `json:"died_incr"`
}

func getRussiaData() (map[string]casesData, error) {
	rusCases := make(map[string]casesData)

	cases := casesData{}
	c := colly.NewCollector()
	c.OnHTML("cv-stats-virus", func(e *colly.HTMLElement) {
		text := e.Attr(":stats-data") // total rus data

		var stat rusTotalStats
		err := json.Unmarshal([]byte(text), &stat)
		if err != nil {
			log.WithFields(log.Fields{"text": text, "err": err}).Error("could not unmarshal russia stats")
			return
		}

		stat.toInt()

		cases.totalCases = stat.sickVal
		cases.newCases = stat.sickChangeVal
		cases.totalDeaths = stat.diedVal
		cases.newDeaths = stat.diedChangeVal
	})

	nnCases := casesData{}
	c.OnHTML("cv-spread-overview", func(e *colly.HTMLElement) {
		text := e.Attr(":spread-data")
		stats := make([]regionStats, 0)
		err := json.Unmarshal([]byte(text), &stats)
		if err != nil {
			log.WithFields(log.Fields{"text": text, "err": err}).Error("could not unmarshal region stats")
			return
		}

		for _, s := range stats {
			if s.Code != nnRegionCode {
				continue
			}

			nnCases.totalCases = s.Sick
			nnCases.newCases = s.SickIncr
			nnCases.totalDeaths = s.Died
			nnCases.newDeaths = s.DiedIncr
		}
	})

	log.Debug("Starting to get russia covid data")
	err := c.Visit("https://xn--80aesfpebagmfblc0a.xn--p1ai/information") // стопкоронавирус.рф
	if err != nil {
		log.Printf("Error! %s\n", err)
	}
	log.WithFields(log.Fields{"totalCases": cases.totalCases, "NNtotalCases": nnCases.totalCases}).Debug("Finished getting russia covid data")
	rusCases["Russia"] = cases
	rusCases[nnID] = nnCases
	return rusCases, err
}
