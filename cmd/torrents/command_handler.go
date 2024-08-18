package main

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gocolly/colly"
	"github.com/hekmon/transmissionrpc/v3"
	"golang.org/x/exp/slog"
)

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
