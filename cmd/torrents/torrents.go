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
	cmdHandler := newCommandHanler(cfg, bot, btclient)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := bot.GetUpdatesChan(updateConfig)

	slog.Info("running", "tgbot.Self.UserName", bot.Self.UserName)
	for update := range updates {
		cmdHandler.handleUpdate(update)
	}
	return nil
}
