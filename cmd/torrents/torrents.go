package main

import (
	"fmt"
	"net/url"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hekmon/transmissionrpc/v3"
	"golang.org/x/exp/slog"
)

func main() {
	cfg, err := readConfig()
	if err != nil {
		slog.Error("cannot read config", "err", err)
		os.Exit(1)
	}

	err = run(cfg)
	slog.Info("run exited", "err", err)
}

func run(cfg config) error {
	bot, err := tgbotapi.NewBotAPI(cfg.token)
	if err != nil {
		return fmt.Errorf("cannot start telegram bot, err: %w", err)
	}

	bturlRaw := fmt.Sprintf("http://transmission:%s@127.0.0.1:9091/transmission/rpc", cfg.transmissionPassword)
	bturl, err := url.Parse(bturlRaw)
	if err != nil {
		return fmt.Errorf("cannot parse transmission url: %w", err)
	}
	btclient, err := transmissionrpc.New(bturl, nil)
	if err != nil {
		return fmt.Errorf("cannot connect to transmission: %w", err)
	}

	cmdHandler := &commandHandler{
		cfg:                cfg,
		tgbot:              bot,
		transmissionClient: btclient,
	}
	routing := make(map[string]messageHandler)
	routing["addtorrent"] = cmdHandler.handleAdd

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := bot.GetUpdatesChan(updateConfig)

	slog.Info("running", "tgbot.Self.UserName", bot.Self.UserName)
	for update := range updates {
		if update.Message == nil {
			slog.Info("received update which is not a message")
			continue
		}

		lgr := slog.Default().With("from.id", update.Message.From.ID, "from.username", update.Message.From.UserName, "chat.id", update.Message.Chat.ID, "chat.name", update.Message.Chat.Title)
		if !cfg.allowedUsers[update.Message.From.ID] {
			lgr.Warn("message from not-allowed user")
			continue
		}

		if !update.Message.IsCommand() {
			lgr.Warn("message is not a command")
			continue
		}

		cmd := update.Message.Command()
		lgr = lgr.With("command", cmd)
		handler, found := routing[cmd]
		if !found {
			lgr.Warn("unknown command")
			replyMsg := tgbotapi.NewMessage(update.Message.Chat.ID, "unknown command")
			replyMsg.ReplyToMessageID = update.Message.MessageID
			_, err := bot.Send(replyMsg)
			if err != nil {
				lgr.Error("cannot send reply", "err", err)
			}
			continue
		}

		err := handler(update.Message, lgr)
		if err != nil {
			lgr.Error("handler error", "err", err)
			replyMsg := tgbotapi.NewMessage(update.Message.Chat.ID, "handler failed")
			replyMsg.ReplyToMessageID = update.Message.MessageID
			_, err := bot.Send(replyMsg)
			if err != nil {
				lgr.Error("cannot send reply", "err", err)
			}
			continue
		}

		replyMsg := tgbotapi.NewMessage(update.Message.Chat.ID, "handler ok!")
		replyMsg.ReplyToMessageID = update.Message.MessageID
		_, err = bot.Send(replyMsg)
		if err != nil {
			lgr.Error("cannot send reply", "err", err)
		}
	}
	return nil
}
