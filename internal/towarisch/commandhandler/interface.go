package cmd

import "gopkg.in/telegram-bot-api.v4"

type Context struct {
	Owners []string // TODO: maybe pass full cfg

	BotMessage bool
}

func NewContext(owners []string) Context {
	ctx := Context{}
	ctx.Owners = append([]string{}, owners...)
	ctx.BotMessage = false
	return ctx
}

type Result struct {
	Reply tgbotapi.Chattable

	BotToStop bool // flag marking that the bot should be stopped
}

func NewResult() Result {
	res := Result{}
	res.Reply = nil
	res.BotToStop = false
	return res
}

type CommandHandler interface {
	HandleMsg(msg *tgbotapi.Update, ctx Context) (*Result, error) // *Result == nil indicates that the handler didn't handle the message
}
