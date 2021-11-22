package yandexnews

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/ilyalavrinov/tgbots/pkg/tgbotutil"
	log "github.com/sirupsen/logrus"
)

type YaNewsTopic int

const (
	YaNewsCovid19 YaNewsTopic = iota
	YaNewsNN      YaNewsTopic = iota
)

var YaNews = map[YaNewsTopic]string{
	YaNewsCovid19: "https://news.yandex.ru/ru/koronavirus5.utf8.js",
	YaNewsNN:      "https://news.yandex.ru/Nizhny_Novgorod/index5.utf8.js",
}

type YaNewsEntry struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Date  string `json:"date"`
	Time  string `json:"time"`
}

func (n YaNewsEntry) ToMarkdown() string {
	return fmt.Sprintf("%s [%s](%s)", tgbotutil.EscapeMarkdown(n.Time), tgbotutil.EscapeMarkdown(n.Title), n.URL)
}

func LoadYaNews(topic YaNewsTopic) ([]YaNewsEntry, error) {
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

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Yandex News returned unexpected code %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("read failed")
		return nil, err
	}

	bodyS := string(body)
	start := strings.IndexByte(bodyS, '[')
	end := strings.IndexByte(bodyS, ']')
	jsontext := body[start : end+1]
	news := make([]YaNewsEntry, 0)
	if err = json.Unmarshal(jsontext, &news); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("read failed")
		return nil, err
	}

	log.WithFields(log.Fields{"topic": topic, "url": url}).Info("news read successfully")
	return news, nil
}
