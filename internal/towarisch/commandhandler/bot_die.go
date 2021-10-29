package cmd

import "log"
import "gopkg.in/telegram-bot-api.v4"

var dieWords = []string{"^умри$", "^die$"}

type botDeathHandler struct{}

func NewDeathHandler() *botDeathHandler {
	return &botDeathHandler{}
}

func (handler *botDeathHandler) HandleMsg(msg *tgbotapi.Update, ctx Context) (*Result, error) {
	if !ctx.BotMessage {
		log.Printf("Message '%s' is not designated for bot manipulation, will not check for bot death", msg.Message.Text)
		return nil, nil
	}

	if ctx.Owners[0] != msg.Message.From.UserName {
		log.Printf("User %s is not in the list of owner, skipping request", msg.Message.From.UserName)
		return nil, nil
	}

	if !msgMatches(msg.Message.Text, dieWords) {
		return nil, nil
	}

	result := NewResult()
	result.BotToStop = true
	return &result, nil
}
