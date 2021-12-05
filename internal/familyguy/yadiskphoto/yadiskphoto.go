package yadiskphoto

import (
	"context"
	"io"
	"math/rand"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ilyalavrinov/tgbots/pkg/tgbotbase"
	log "github.com/sirupsen/logrus"
	"github.com/studio-b12/gowebdav"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

type dailyPhoto struct {
	tgbotbase.BaseHandler

	cron  tgbotbase.Cron
	props tgbotbase.PropertyStorage
}

var _ tgbotbase.BackgroundMessageHandler = &dailyPhoto{}

func NewDailyPhoto(cron tgbotbase.Cron, props tgbotbase.PropertyStorage) tgbotbase.BackgroundMessageHandler {
	return &dailyPhoto{
		cron:  cron,
		props: props,
	}
}

func (h *dailyPhoto) Init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- tgbotbase.ServiceMsg) {
	h.OutMsgCh = outMsgCh
}

func (h *dailyPhoto) Name() string {
	return "daily photo"
}

func (h *dailyPhoto) Run() {
	ctx := context.TODO()
	props, _ := h.props.GetEveryHavingProperty(ctx, "yadiskDailyPhotoTime")
	for _, prop := range props {
		if (prop.User != 0) && (tgbotbase.ChatID(prop.User) != prop.Chat) {
			log.Printf("Skipping special setting for user %d in chat %d", prop.User, prop.Chat)
			continue
		}
		dur, err := time.ParseDuration(prop.Value)
		if err != nil {
			log.Printf("Could not parse duration %s for chat %d due to error: %s", prop.Value, prop.Chat, err)
			continue
		}

		propRoot, err := h.props.GetProperty(ctx, "yadiskDailyPhotoRoot", 0, prop.Chat)
		if err != nil {
			log.WithFields(log.Fields{"err": err, "chat": prop.Chat}).Error("could not get root dir property")
			continue
		}

		propUsername, err := h.props.GetProperty(ctx, "yadiskDailyPhotoUsername", 0, prop.Chat)
		if err != nil {
			log.WithFields(log.Fields{"err": err, "chat": prop.Chat}).Error("could not get username property")
			continue
		}

		propPassword, err := h.props.GetProperty(ctx, "yadiskDailyPhotoPassword", 0, prop.Chat)
		if err != nil {
			log.WithFields(log.Fields{"err": err, "chat": prop.Chat}).Error("could not get password property")
			continue
		}

		when := tgbotbase.CalcNextTimeFromMidnight(time.Now(), dur)
		job := dailyPhotoJob{
			chatID:   prop.Chat,
			rootPath: propRoot,
			username: propUsername,
			password: propPassword,
		}
		job.OutMsgCh = h.OutMsgCh

		h.cron.AddJob(when, &job)
	}
}

type dailyPhotoJob struct {
	tgbotbase.BaseHandler
	chatID tgbotbase.ChatID

	rootPath string
	username string
	password string
}

var _ tgbotbase.CronJob = &dailyPhotoJob{}

func (job *dailyPhotoJob) Do(scheduledWhen time.Time, cron tgbotbase.Cron) {
	defer cron.AddJob(scheduledWhen.Add(24*time.Hour), job)

	client := gowebdav.NewClient("https://webdav.yandex.ru", job.username, job.password)
	files := getFileList(client, job.rootPath)
	if len(files) == 0 {
		log.WithFields(log.Fields{"path": job.rootPath}).Error("empty list of files")
		return
	}

	targetFile := files[rand.Intn(len(files))]
	remote, err := client.ReadStream(targetFile)
	if err != nil {
		log.WithFields(log.Fields{"file": targetFile, "error": err}).Error("Could not open remote file for reading")
		return
	}

	tmp, err := os.CreateTemp("", "yadiskDailyPhoto_")

	_, err = io.Copy(tmp, remote)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Could not copy remote file into a local")
		return
	}

	job.OutMsgCh <- tgbotapi.NewPhotoUpload(int64(job.chatID), tmp.Name())
}

func getFileList(client *gowebdav.Client, fpath string) []string {
	var result []string
	fstat, err := client.Stat(fpath)
	if err != nil {
		log.WithFields(log.Fields{"err": err, "path": fpath}).Error("Cannot get remote path stats")
		return result
	}

	if fstat.IsDir() {
		dirContents, err := client.ReadDir(fpath)
		if err != nil {
			log.WithFields(log.Fields{"err": err, "path": fpath}).Error("Could not read remote directory contents")
			return result
		}
		for _, finfo := range dirContents {
			result = append(result, getFileList(client, path.Join(fpath, finfo.Name()))...)
		}
	} else {
		if strings.HasSuffix(strings.ToLower(fpath), ".jpg") {
			return []string{fpath}
		}
	}

	return result
}
