package covid

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
	"github.com/ilyalavrinov/tgbots/pkg/tgbotbase"
	log "github.com/sirupsen/logrus"
)

type casesData struct {
	date            time.Time
	newCases        int
	newCasesGrowth  int
	totalCases      int
	newDeaths       int
	newDeathsGrowth int
	totalDeaths     int
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

	data := make(map[string]casesData, 200)

	intlData, err := getInternationalData()
	if err == nil {
		for key, val := range intlData {
			data[key] = val
		}
	}

	russiaData, err := getRussiaData()
	if err == nil {
		for key, val := range russiaData {
			data[key] = val
		}
	}

	j.updates <- covidData{
		countryRaw:    nil,
		countryLatest: data,
	}
}

func getInternationalData() (map[string]casesData, error) {
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

	r := csv.NewReader(f)
	data, err := r.ReadAll()
	if err != nil {
		log.Printf("Could not read covid info at %q, err: %s", fname, err)
		return nil, err
	}

	raw := make(map[string][]casesData, 200)
	latest := make(map[string]casesData, 200)
	dates := make(map[string]time.Time, 200)

	var prevDayInfo casesData

	for _, line := range data {
		d, _ := time.Parse("2006-01-02", line[colDate])

		newCases, _ := strconv.Atoi(line[colNewCases])
		totalCases, _ := strconv.Atoi(line[colTotalCases])
		newDeaths, _ := strconv.Atoi(line[colNewDeaths])
		totalDeaths, _ := strconv.Atoi(line[colTotalDeaths])
		cinfo := casesData{
			date:            d,
			newCases:        newCases,
			newCasesGrowth:  newCases - prevDayInfo.newCases,
			totalCases:      totalCases,
			newDeaths:       newDeaths,
			newDeathsGrowth: prevDayInfo.newDeaths,
			totalDeaths:     totalDeaths,
		}

		prevDayInfo = cinfo

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

	return latest, nil
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
