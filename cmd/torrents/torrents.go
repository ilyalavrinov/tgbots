package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gocolly/colly"
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

type config struct {
	allowedUsers         map[int64]bool
	token                string
	transmissionPassword string
}

func readConfig() (config, error) {
	users := os.Getenv("TGTORRENTSBOT_USERS")
	usersSeparated := strings.Split(users, ",")
	if len(users) == 0 {
		return config{}, fmt.Errorf("no allowed users found")
	}

	tgtoken := os.Getenv("TGTORRENTSBOT_TOKEN")
	if tgtoken == "" {
		return config{}, fmt.Errorf("no token found")
	}

	transmissionPWD := os.Getenv("TGTORRENTSBOT_TRANSMISSION_PASSWORD")
	if transmissionPWD == "" {
		return config{}, fmt.Errorf("transmission password not set")
	}

	allowedUsers := make(map[int64]bool, len(usersSeparated))
	for _, u := range usersSeparated {
		id, err := strconv.ParseInt(u, 10, 64)
		if err != nil {
			return config{}, fmt.Errorf("cannot convert user %s to int64 id: %w", u, err)
		}
		allowedUsers[id] = true
	}

	return config{
		allowedUsers:         allowedUsers,
		token:                tgtoken,
		transmissionPassword: transmissionPWD,
	}, nil
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

type commandHandler struct {
	cfg                config
	tgbot              *tgbotapi.BotAPI
	transmissionClient *transmissionrpc.Client
}

type messageHandler func(msg *tgbotapi.Message, lgr *slog.Logger) error

func (h *commandHandler) handleAdd(msg *tgbotapi.Message, lgr *slog.Logger) error {
	pageUrl := msg.CommandArguments()
	magnetLink, err := getRutrackerMagnetURL(pageUrl)
	if err != nil {
		return err
	}
	lgr.Info("parsed rutracker", "pageUrl", pageUrl, "magnetLink", magnetLink)

	torrent, err := h.transmissionClient.TorrentAdd(context.TODO(), transmissionrpc.TorrentAddPayload{Filename: &magnetLink})
	if err != nil {
		return fmt.Errorf("cannot add torrent to transmission: %w", err)
	}

	lgr.Info("torrent added", "torrent.Name", torrent.Name)
	return nil
}

func getRutrackerMagnetURL(pageUrl string) (string, error) {
	var magnetLink string
	c := colly.NewCollector()
	c.OnHTML("a", func(e *colly.HTMLElement) {
		if e.Attr("class") != "magnet-link" {
			return
		}
		magnetLink = e.Attr("href")
	})
	err := c.Visit(pageUrl)
	if err != nil {
		return "", fmt.Errorf("cannot crawl at %q, err: %w", pageUrl, err)
	}
	if magnetLink == "" {
		return "", fmt.Errorf("cannot find magnet link at %q", pageUrl)
	}
	return magnetLink, err
}
