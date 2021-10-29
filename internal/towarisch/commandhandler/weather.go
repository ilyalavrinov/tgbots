package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/admirallarimda/tgbotbase"

	"github.com/go-redis/redis"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

var reToday *regexp.Regexp = regexp.MustCompile("сегодня")
var reDayAfterTomorrow *regexp.Regexp = regexp.MustCompile("послезавтра")
var reTomorrow *regexp.Regexp = regexp.MustCompile("завтра")

func requestData(reqType string, cityId int64, apiKey string) ([]byte, error) {
	weather_url := fmt.Sprintf("http://api.openweathermap.org/data/2.5/%s?id=%d&APPID=%s&lang=ru&units=metric", reqType,
		cityId,
		apiKey)
	log.Printf("Sending weather request using url: %s", weather_url)

	resp, err := http.Get(weather_url)
	if err != nil {
		log.Printf("Could not get data from '%s' due to error: %s", weather_url, err)
		return []byte{}, err
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Could not read response body from '%s' due to error: %s", weather_url, err)
		return []byte{}, err
	}

	log.Printf("Weather response: %s", string(bodyBytes))

	return bodyBytes, nil
}

func (h *weatherHandler) determineCity(msg tgbotapi.Message) (int64, error) {
	reInCity := regexp.MustCompile("(в|in) ([\\wA-Za-zА-Яа-я]+)")
	text := msg.Text
	if reInCity.MatchString(text) {
		log.Printf("Message '%s' matches 'in city' regexp %s", text, reInCity)
		matches := reInCity.FindStringSubmatch(text)
		city := matches[2]
		return getCityIDByName(h.redisconn, city)
	}

	return getCityIDFromProperty(h.properties, h.redisconn, tgbotbase.UserID(msg.From.ID), tgbotbase.ChatID(msg.Chat.ID))
}

func getCityIDFromProperty(props tgbotbase.PropertyStorage, conn *redis.Client, userID tgbotbase.UserID, chatID tgbotbase.ChatID) (int64, error) {
	city, err := props.GetProperty("city", userID, chatID)
	if err != nil {
		log.Printf("Could not get weather city property due to error: %s", err)
		return 0, err
	}

	return getCityIDByName(conn, city)
}

func getCityIDByName(conn *redis.Client, city string) (int64, error) {
	city = strings.ToLower(city)

	key := fmt.Sprintf("openweathermap:city:%s", city)
	result := conn.HGet(key, "id")
	if result.Err() != nil {
		log.Printf("Could not HGet for key '%s', error: %s", key, result.Err())
		return 0, result.Err()
	}

	cityId, err := result.Int64()
	if err != nil {
		log.Printf("Could not convert ID for key '%s' into int, error: %s", key, err)
		return 0, err
	}
	log.Printf("City ID for %s is %d", city, cityId)

	return cityId, nil
}

func determineDate(text string) *time.Time {
	now := time.Now()
	target_day := now

	if reDayAfterTomorrow.MatchString(text) { // DayAfterTomorrow should go first as simple Tomorrow is a substring
		log.Printf("Forecast is requested for the day after tomorrow")
		target_day = time.Date(now.Year(), now.Month(), now.Day()+2,
			0, 0, 0, 0, time.Local)
	} else if reTomorrow.MatchString(text) {
		log.Printf("Forecast is requested for tomorrow")
		target_day = time.Date(now.Year(), now.Month(), now.Day()+1,
			0, 0, 0, 0, time.Local)
	} else if reToday.MatchString(text) {
		log.Printf("Forecast is requested for today")
		target_day = time.Date(now.Year(), now.Month(), now.Day(),
			0, 0, 0, 0, time.Local)
	}

	var result *time.Time = nil
	if target_day != now { // probably 'is_regex_matched' flag is better
		result = &target_day
	}
	return result
}

type weatherData struct {
	Cod  int
	Main struct {
		Temp float64
	}
	Name    string
	Weather []struct {
		Description string
	}
	Wind struct {
		Speed float32
	}
}

type forecastData struct {
	City struct {
		Name string
	}
	List []struct {
		DT_txt  string
		Weather []struct {
			Description string
		}
		Main struct {
			Temp float32
		}
	}
}

func getCurrentWeather(token string, cityId int64) (string, error) {
	bytes, err := requestData("weather", cityId, token)
	if err != nil {
		return "", nil
	}

	weather_data := weatherData{}
	err = json.Unmarshal(bytes, &weather_data)
	//err = json.NewDecoder(resp.Body).Decode(&weather_data)
	if err != nil || weather_data.Cod != 200 {
		return "Я не смог распарсить погоду :(", err
	}

	weather_msg := fmt.Sprintf("Сейчас в %s: %s, %.1f градусов, дует ветер %.0f м/с", weather_data.Name,
		weather_data.Weather[0].Description,
		weather_data.Main.Temp,
		weather_data.Wind.Speed)
	return weather_msg, nil
}

func getForecast(token string, cityId int64, date time.Time) (string, error) {
	log.Printf("Checking for upcoming weather in city %d", cityId)
	bytes, err := requestData("forecast", cityId, token)
	if err != nil {
		return "", err
	}

	forecast_data := forecastData{}
	err = json.Unmarshal(bytes, &forecast_data)
	//err = json.NewDecoder(resp.Body).Decode(&weather_data)
	if err != nil {
		return "Я не смог распарсить прогноз :(", err
	}

	now := time.Now()
	if (date == now) && (date.Hour() > 18) {
		return "Иди спи, нечего гулять по ночам", nil
	}

	forecast_start := date
	if forecast_start.Hour() < 6 {
		forecast_start = time.Date(forecast_start.Year(), forecast_start.Month(), forecast_start.Day(),
			5, 59, 0, 0, time.Local)
	}
	forecast_end := time.Date(forecast_start.Year(), forecast_start.Month(), forecast_start.Day(),
		18, 01, 00, 0, time.Local)

	forecasts := make([]string, 0, 5)
	for _, val := range forecast_data.List {
		t, err := time.Parse(timeFormat_API, val.DT_txt)
		if err != nil {
			log.Printf("Error while parsing date: %s; error: %s", val.DT_txt, err)
			continue
		}
		t = t.Local()
		if t.Before(forecast_start) || t.After(forecast_end) {
			log.Printf("Skipping date: %s", t)
			continue
		}
		log.Printf("Forecast: %s,t = %.1f, %s", t, val.Main.Temp, val.Weather[0].Description)
		forecasts = append(forecasts, fmt.Sprintf("%s: %.1f\u2103, %s", t.Format(timeFormat_Out_Time), val.Main.Temp, val.Weather[0].Description))
	}

	if len(forecasts) == 0 {
		log.Printf("Something went wrong - no forecast")
		return "Я не смог сделать прогноз :(", err
	}

	forecast_msg := fmt.Sprintf("Прогнозирую на %s в %s:\n", date.Format(timeFormat_Out_Date), forecast_data.City.Name)
	for _, forecast := range forecasts {
		forecast_msg += forecast
		forecast_msg += "\n"
	}

	return forecast_msg, nil
}

const timeFormat_API = "2006-01-02 15:04:05"
const timeFormat_Out_Date = "Mon, 02 Jan"
const timeFormat_Out_Time = "15:04"

var weatherWords = []string{"^погода", "^weather"}

type weatherHandler struct {
	tgbotbase.BaseHandler
	token      string
	redisconn  *redis.Client
	properties tgbotbase.PropertyStorage
}

func NewWeatherHandler(token string, pool tgbotbase.RedisPool, properties tgbotbase.PropertyStorage) tgbotbase.IncomingMessageHandler {
	handler := weatherHandler{}
	handler.token = token
	handler.redisconn = pool.GetConnByName("openweathermap")
	handler.properties = properties
	if handler.redisconn == nil {
		log.Panicf("Could not get connection to Redis")
	}
	return &handler
}

func (h *weatherHandler) Init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- tgbotbase.ServiceMsg) tgbotbase.HandlerTrigger {
	h.OutMsgCh = outMsgCh
	return tgbotbase.NewHandlerTrigger(regexp.MustCompile("^погода"), nil)
}

func (h *weatherHandler) Name() string {
	return "weather"
}

func (h *weatherHandler) HandleOne(msg tgbotapi.Message) {
	text := msg.Text

	date := determineDate(text)
	cityID, err := h.determineCity(msg)
	if err != nil {
		log.Printf("Could not determine city from message '%s' due to error: '%s'", text, err)

		reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("Не смог распарсить город :("))
		reply.BaseChat.ReplyToMessageID = msg.MessageID
		h.OutMsgCh <- reply
		return
	}

	var replyMsg string

	if date == nil {
		replyMsg, err = getCurrentWeather(h.token, cityID)
	} else {
		replyMsg, err = getForecast(h.token, cityID, *date)
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, replyMsg)
	reply.BaseChat.ReplyToMessageID = msg.MessageID
	h.OutMsgCh <- reply
}
