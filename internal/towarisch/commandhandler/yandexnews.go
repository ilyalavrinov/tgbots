package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

type YaNewsTopic int

const (
	YaNewsCovid19 YaNewsTopic = iota
	YaNewsNN      YaNewsTopic = iota
)

var YaNews = map[YaNewsTopic]string{
	YaNewsCovid19: "https://news.yandex.by/ru/koronavirus5.utf8.js",
	YaNewsNN:      "https://news.yandex.by/Nizhny_Novgorod/index5.utf8.js",
}

type yaNewsEntry struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Date  string `json:"date"`
	Time  string `json:"time"`
}

func (n yaNewsEntry) toMarkdown() string {
	return fmt.Sprintf("%s [%s](%s)", escapeMarkdownSpecial(n.Time), escapeMarkdownSpecial(n.Title), n.URL)
}

func loadYaNews(topic YaNewsTopic) ([]yaNewsEntry, error) {
	url, found := YaNews[topic]
	if !found {
		log.WithFields(log.Fields{"topic": topic}).Error("unknown news topic")
		return nil, errors.New("unknown news topic")
	}

	resp, err := http.Get(url)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "url": url}).Error("get failed")
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("read failed")
		return nil, err
	}

	bodyS := string(body)
	start := strings.IndexByte(bodyS, '[')
	end := strings.IndexByte(bodyS, ']')
	jsontext := body[start : end+1]
	news := make([]yaNewsEntry, 0)
	if err = json.Unmarshal(jsontext, &news); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("read failed")
		return nil, err
	}

	log.WithFields(log.Fields{"topic": topic, "url": url}).Info("news read successfully")
	return news, nil
}
