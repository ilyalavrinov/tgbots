package covid

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/ilyalavrinov/tgbots/pkg/tgbotbase"
	log "github.com/sirupsen/logrus"
)

type covidUpdateJob struct {
	updates chan<- History
	history History
}

func (j *covidUpdateJob) Do(scheduledWhen time.Time, cron tgbotbase.Cron) {
	defer cron.AddJob(scheduledWhen.Add(30*time.Minute), j)

	log.Debug("Starting update of covid data")
	/* TODO: it loads too old data, needs rework
	_, err := getInternationalData(j.history)
	if err != nil {
		log.WithField("err", err).Error("cloud not get international data")
		return
	}
	*/

	russiaData, err := getRussiaData(j.history)
	if err != nil {
		log.WithField("err", err).Error("cloud not get russia data")
		return
	}

	log.WithField("changes", len(russiaData)).Debug("Checking if update is to be sent")
	if len(russiaData) > 0 { // currently we care only if russiadata get updated
		j.updates <- j.history
	}
}

const (
	colDate        = 0
	colCountry     = 1
	colNewCases    = 2
	colNewDeaths   = 3
	colTotalCases  = 4
	colTotalDeaths = 5
)

func getInternationalData(h History) (map[string]bool, error) {
	log.Debug("Start covid update")
	url := "https://covid.ourworldindata.org/data/ecdc/full_data.csv"
	fpath := path.Join("/tmp", "ilya-tgbot", "covid")
	if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
		log.Printf("Could not create covid directories at %q, err: %s", fpath, err)
		return nil, err
	}
	fname := path.Join(fpath, fmt.Sprintf("cases-%s.csv", time.Now().Format("20060102150405")))
	if err := downloadFile(fname, url); err != nil {
		log.Printf("Could not download covid info from %q to %q, err: %s", url, fname, err)
		return nil, err
	}

	f, err := os.Open(fname)
	if err != nil {
		log.Printf("Could not open covid info at %q, err: %s", fname, err)
		return nil, err
	}
	defer os.Remove(fname)

	r := csv.NewReader(f)
	data, err := r.ReadAll()
	if err != nil {
		log.Printf("Could not read covid info at %q, err: %s", fname, err)
		return nil, err
	}

	headerRead := false
	for _, line := range data {
		// TODO: dirty hack to skip header. Use csvreader properly instead
		if !headerRead {
			headerRead = true
			continue
		}
		d, _ := time.Parse("2006-01-02", line[colDate])

		_, err := h.addIfNotExist(context.TODO(), line[colCountry], d, atoi(line[colTotalCases]), atoi(line[colTotalDeaths]))
		if err != nil {
			log.WithFields(log.Fields{"err": err, "location": line[colCountry]}).Error("cloud not save data")
			continue
		}
	}

	// I don't care now fow internation data updates - no indication for its update for now
	return nil, nil
}

func statToInt(s string) int {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "+", "")

	return atoi(s)
}

type chartDayData struct {
	Date    string
	DateVal time.Time
	Sick    int
	Healed  int
	Died    int
}

func (d *chartDayData) convert() {
	d.DateVal, _ = time.Parse("02.01.2006", d.Date)
}

const (
	locationRussia          = "RU-TOTAL"
	locationRussiaNN        = "RU-NIZ"
	locationRussiaMSK       = "RU-MOW"
	locationRussiaMSKRegion = "RU-MOS"
	locationRussiaSPb       = "RU-SPE"
	locationRussiaSPbRegion = "RU-LEN"
)

func getRussiaData(h History) (map[string]bool, error) {
	locationsUpdated := make(map[string]bool, 0)
	var latestKnownDate time.Time
	var latestUpdate time.Time
	var crawlError error

	c := colly.NewCollector()
	c.OnHTML("cv-stats-virus", func(e *colly.HTMLElement) {
		text := e.Attr(":charts-data") // per day distribution
		var chartData []chartDayData
		err := json.Unmarshal([]byte(text), &chartData)
		if err != nil {
			log.WithFields(log.Fields{"text": text, "err": err}).Error("could not unmarshal russia per-day stats")
			crawlError = err
			return
		}

		log.WithField("days", len(chartData)).Debug("read covid data for russia")
		for _, d := range chartData {
			d.convert()

			if d.DateVal.After(latestKnownDate) {
				latestKnownDate = d.DateVal
			}

			added, err := h.addIfNotExist(context.TODO(), locationRussia, d.DateVal, d.Sick, d.Died)
			if err != nil {
				log.WithFields(log.Fields{"err": err}).Error("could not save russia data to history")
				crawlError = err
				continue
			}
			if added && d.DateVal.After(latestUpdate) {
				latestUpdate = d.DateVal
			}
		}
	})

	c.OnHTML("cv-spread-overview", func(e *colly.HTMLElement) {
		if latestKnownDate.IsZero() {
			panic("russia covid side parsing called in wrong order")
		}
		if !latestUpdate.Equal(latestKnownDate) { // we haven't got the latest day update, i.e. we already know most recent data and notified everyone
			log.WithFields(log.Fields{"lastKnownDate": latestKnownDate, "lastUpdate": latestUpdate}).Debug("All data is known, skipping region update")
			return
		}
		locationsUpdated[locationRussia] = true

		text := e.Attr(":spread-data")

		type regionStats struct {
			Code     string
			Sick     int
			Died     int
			SickIncr int `json:"sick_incr"`
			DiedIncr int `json:"died_incr"`
		}
		stats := make([]regionStats, 0)

		err := json.Unmarshal([]byte(text), &stats)
		if err != nil {
			log.WithFields(log.Fields{"text": text, "err": err}).Error("could not unmarshal region stats")
			crawlError = err
			return
		}

		log.WithField("regions", len(stats)).Debug("read covid data for regions")
		for _, s := range stats {
			added, err := h.addIfNotExist(context.TODO(), s.Code, latestKnownDate, s.Sick, s.Died)
			if err != nil {
				log.WithFields(log.Fields{"location": s.Code, "err": err}).Error("could not save history for region")
				continue
			}

			if added {
				locationsUpdated[s.Code] = true
			}
		}
	})

	log.Debug("Starting to get russia covid data")
	err := c.Visit("https://xn--80aesfpebagmfblc0a.xn--p1ai/information") // стопкоронавирус.рф
	if err != nil {
		log.Printf("Error! %s\n", err)
	}
	if crawlError != nil {
		log.Printf("Crawl Error! %s\n", crawlError)
	}
	return locationsUpdated, err
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
