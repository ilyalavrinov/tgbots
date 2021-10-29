package cmd

import "time"
import "github.com/admirallarimda/tgbotbase"

type Reminder struct {
	t       time.Time
	chat    tgbotbase.ChatID
	replyTo int // message ID
}

type ReminderStorage interface {
	AddReminder(Reminder)
	RemoveReminder(Reminder)
	LoadAll() []Reminder
}
