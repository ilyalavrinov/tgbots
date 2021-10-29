package cmd

import "log"
import "time"
import "strings"
import "os"
import "io"
import "path"
import "net/http"
import "gopkg.in/telegram-bot-api.v4"
import "github.com/admirallarimda/tgbotbase"

type kittiesHandler struct {
	tgbotbase.BaseHandler
	properties tgbotbase.PropertyStorage
	cron       tgbotbase.Cron
}

func NewKittiesHandler(cron tgbotbase.Cron, properties tgbotbase.PropertyStorage) tgbotbase.BackgroundMessageHandler {
	handler := kittiesHandler{
		properties: properties,
		cron:       cron}
	return &handler
}

func (h *kittiesHandler) Init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- tgbotbase.ServiceMsg) {
	h.OutMsgCh = outMsgCh
}

func (h *kittiesHandler) Name() string {
	return "morning kitties"
}

func (h *kittiesHandler) Run() {
	now := time.Now()
	props, _ := h.properties.GetEveryHavingProperty("catTime")
	for _, prop := range props {
		if (prop.User != 0) && (tgbotbase.ChatID(prop.User) != prop.Chat) {
			log.Printf("Morning kitties: Skipping special setting for user %d in chat %d", prop.User, prop.Chat)
			continue
		}
		dur, err := time.ParseDuration(prop.Value)
		if err != nil {
			log.Printf("Could not parse duration %s for chat %d due to error: %s", prop.Value, prop.Chat, err)
			continue
		}
		when := tgbotbase.CalcNextTimeFromMidnight(now, dur)
		job := kittiesJob{chatID: prop.Chat}
		job.OutMsgCh = h.OutMsgCh
		h.cron.AddJob(when, &job)
	}
}

type kittiesJob struct {
	tgbotbase.BaseHandler
	chatID tgbotbase.ChatID
}

func (job *kittiesJob) Do(scheduledWhen time.Time, cron tgbotbase.Cron) {
	defer cron.AddJob(scheduledWhen.Add(24*time.Hour), job)
	const url = "http://thecatapi.com/api/images/get?format=src&type=jpg"

	log.Printf("Preparing to load new catpic using %s", url)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error has been occured during loading cat: %s. Aborting loading", err)
		return
	}
	defer resp.Body.Close()

	// TODO: via filecache or something
	actualURL := resp.Request.URL.String()
	log.Printf("Cat received from %s", actualURL)
	actualURLParts := strings.Split(actualURL, "/")
	filename := actualURLParts[len(actualURLParts)-1] // getting last piece as actual filename
	fpath := path.Join("/tmp", filename)
	file, err := os.Create(fpath)
	if err != nil {
		log.Printf("Could not create new file for a cat %s due to error: %s. Skipping this one", filename, err)
		return
	}
	// Use io.Copy to just dump the response body to the file. This supports huge files
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		// TODO: remove created file
		log.Printf("Could not store a catpic from the Internet to %s due to error: %s", filename, err)
		return
	}
	file.Close()

	picMsg := tgbotapi.NewPhotoUpload(int64(job.chatID), fpath)
	picMsg.Caption = "утренний котик!"

	job.OutMsgCh <- picMsg
}
