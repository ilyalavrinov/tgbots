package cmd

import "github.com/admirallarimda/tgbotbase"
import "log"
import "regexp"
import "time"
import "strconv"
import "errors"
import "fmt"
import "gopkg.in/telegram-bot-api.v4"

const timeFormat_Out_Confirm = "2006-01-02 15:04:05 MST"

type remindCronJob struct {
	outMsgCh chan<- tgbotapi.Chattable
	storage  ReminderStorage

	reminder Reminder
}

func newRemindCronJob(storage ReminderStorage, outMsgCh chan<- tgbotapi.Chattable, reminder Reminder) remindCronJob {
	job := remindCronJob{
		outMsgCh: outMsgCh,
		storage:  storage,
		reminder: reminder}
	storage.AddReminder(reminder)
	return job
}

func (j *remindCronJob) Do(scheduled time.Time, cron tgbotbase.Cron) {
	msg := tgbotapi.NewMessage(int64(j.reminder.chat), "Напоминаю")
	msg.BaseChat.ReplyToMessageID = j.reminder.replyTo

	j.outMsgCh <- msg
	j.storage.RemoveReminder(j.reminder)
}

type remindHandler struct {
	tgbotbase.BaseHandler
	cron       tgbotbase.Cron
	storage    ReminderStorage
	properties tgbotbase.PropertyStorage
}

func NewRemindHandler(cron tgbotbase.Cron, storage ReminderStorage, properties tgbotbase.PropertyStorage) *remindHandler {
	handler := &remindHandler{
		cron:       cron,
		storage:    storage,
		properties: properties}

	return handler
}

func determineReminderTime(msg string) (time.Time, error) {
	reAfter := regexp.MustCompile("через (\\d*) *([\\wа-я]+)")
	// TODO: uncomment during implementation
	//reAt := regexp.MustCompile("в (\\d{1,2}):(\\d{1,2})")
	//reTomorrow := regexp.MustCompile("завтра")
	//reDayAfterTomorrow := regexp.MustCompile("послезавтра")
	// TODO: add days of week parsing

	now := time.Now()
	if reAfter.MatchString(msg) {
		log.Printf("Message '%s' matches 'after' regexp %s", msg, reAfter)
		matches := reAfter.FindStringSubmatch(msg)
		timeQuantity := matches[1] // (\d+)
		timePeriod := matches[2]   // ([\wа-я]+)

		log.Printf("Reminder command matched: quantity '%s' period '%s'", timeQuantity, timePeriod)

		var q int = 1
		if len(timeQuantity) > 0 {
			q, _ = strconv.Atoi(timeQuantity)
		}
		period := time.Minute
		matchedSecond, _ := regexp.MatchString("секунд", timePeriod)
		matchedMinute, _ := regexp.MatchString("минут", timePeriod)
		matchedHour, _ := regexp.MatchString("час", timePeriod)
		matchedDay, _ := regexp.MatchString("день|дня|дней", timePeriod)
		matchedWeek, _ := regexp.MatchString("недел", timePeriod)
		matchedMonth, _ := regexp.MatchString("месяц", timePeriod)
		matchedYear, _ := regexp.MatchString("год|лет", timePeriod)
		if matchedSecond {
			period = time.Second
		} else if matchedMinute {
			period = time.Minute
		} else if matchedHour {
			period = time.Hour
		} else if matchedDay {
			period = 24 * time.Hour
		} else if matchedWeek {
			period = 7 * 24 * time.Hour
		} else if matchedMonth {
			period = 30 * 24 * time.Hour
		} else if matchedYear {
			period = 365 * 24 * time.Hour
		} else {
			log.Printf("Time period %s doesn't match any known format", timePeriod)
			err := errors.New("Time period doesn't match any known")
			return now, err
		}

		return now.Add(period * time.Duration(q)), nil
	}

	return now, nil
}

func (h *remindHandler) HandleOne(msg tgbotapi.Message) {
	t, err := determineReminderTime(msg.Text)
	if err != nil {
		log.Printf("Could not determine time from message '%s' with error: %s", msg.Text, err)
	}

	job := newRemindCronJob(h.storage, h.OutMsgCh, Reminder{
		chat:    tgbotbase.ChatID(msg.Chat.ID),
		replyTo: msg.MessageID,
		t:       t})
	h.cron.AddJob(t, &job)

	tz, _ := h.properties.GetProperty("timezone", tgbotbase.UserID(msg.From.ID), tgbotbase.ChatID(msg.Chat.ID))
	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Printf("Could not load timezone %s correctly; location loaded with error: %s", tz, err)
	} else {
		t = t.In(loc)
	}

	replyText := fmt.Sprintf("Принято, напомню около %s", t.Format(timeFormat_Out_Confirm))
	replyMsg := tgbotapi.NewMessage(msg.Chat.ID, replyText)
	replyMsg.BaseChat.ReplyToMessageID = msg.MessageID
	h.OutMsgCh <- replyMsg

}

func (h *remindHandler) Init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- tgbotbase.ServiceMsg) tgbotbase.HandlerTrigger {
	h.OutMsgCh = outMsgCh

	allReminders := h.storage.LoadAll()
	for _, r := range allReminders {
		job := newRemindCronJob(h.storage, outMsgCh, r)
		h.cron.AddJob(r.t, &job)
	}

	return tgbotbase.NewHandlerTrigger(nil, []string{"remind", "todo"})
}

func (h *remindHandler) Name() string {
	return "reminder"
}
