package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gocolly/colly"
	"github.com/hekmon/cunits/v2"
	"github.com/hekmon/transmissionrpc/v3"
	"golang.org/x/exp/slog"
	"golang.org/x/sys/unix"
)

type commandHandler struct {
	cfg                config
	tgbot              *tgbotapi.BotAPI
	transmissionClient *transmissionrpc.Client

	outCh chan tgbotapi.Chattable
}

func newCommandHanler(cfg config, tgbot *tgbotapi.BotAPI, btclient *transmissionrpc.Client) *commandHandler {
	h := &commandHandler{
		cfg:                cfg,
		tgbot:              tgbot,
		transmissionClient: btclient,
		outCh:              make(chan tgbotapi.Chattable),
	}

	go h.sendReplies()

	return h
}

func (h *commandHandler) sendReplies() {
	for outMsg := range h.outCh {
		msg, err := h.tgbot.Send(outMsg)
		if err != nil {
			slog.Error("cannot send reply", "err", err, "msg.Chat.ID", msg.Chat.ID, "msg.Chat.UserName", msg.Chat.UserName, "msg.ReplyToMessage.MessageID", msg.ReplyToMessage.MessageID)
		}
	}
}

func (h *commandHandler) handleUpdate(update tgbotapi.Update) {
	if update.Message == nil {
		slog.Info("received update which is not a message")
		return
	}

	lgr := slog.Default().With("from.id", update.Message.From.ID, "from.username", update.Message.From.UserName, "chat.id", update.Message.Chat.ID, "chat.name", update.Message.Chat.Title)
	if !h.cfg.allowedUsers[update.Message.From.ID] {
		lgr.Warn("message from not-allowed user")
		return
	}

	if !update.Message.IsCommand() {
		lgr.Warn("message is not a command")
		return
	}

	cmd := update.Message.Command()
	lgr = lgr.With("command", cmd)
	var handlerErr error
	switch cmd {
	case "add", "addtorrent":
		handlerErr = h.handleAdd(update.Message, lgr)
	case "stats":
		handlerErr = h.handleStats(update.Message, lgr)
	case "list", "listtorrents":
		handlerErr = h.handleList(update.Message, lgr)
	default:
		lgr.Warn("unknown command")
		replyMsg := tgbotapi.NewMessage(update.Message.Chat.ID, "unknown command")
		replyMsg.ReplyToMessageID = update.Message.MessageID
		h.outCh <- replyMsg
		return
	}

	if handlerErr != nil {
		lgr.Error("handler error", "err", handlerErr)
		replyMsg := tgbotapi.NewMessage(update.Message.Chat.ID, "oops, something went wrong")
		replyMsg.ReplyToMessageID = update.Message.MessageID
	}
}

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

	lgr.Info("torrent added", "torrent.Name", *torrent.Name)
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

func (h *commandHandler) handleStats(msg *tgbotapi.Message, lgr *slog.Logger) error {
	stats, err := h.transmissionClient.SessionStats(context.TODO())
	if err != nil {
		return fmt.Errorf("stats failed: %w", err)
	}

	diskToCheck := "/"
	diskAvailMem := "NA"
	var stat unix.Statfs_t
	err = unix.Statfs(diskToCheck, &stat)
	if err != nil {
		lgr.Error("dir stat failed", "dir", diskToCheck, "err", err)
	} else {
		diskAvailMem = cunits.ImportInByte(float64(stat.Bavail * uint64(stat.Bsize))).GetHumanSizeRepresentation()
	}

	statsText := fmt.Sprintf(`session stats:
Active Torrent Count: %d; Total Torrent Count: %d
Bytes Downloaded: %s; Bytes Uploaded: %s
Disk free space: %s`,
		stats.ActiveTorrentCount, stats.TorrentCount,
		stats.CurrentStats.GetDownloaded(), stats.CurrentStats.GetUploaded(),
		diskAvailMem)
	reply := tgbotapi.NewMessage(msg.Chat.ID, statsText)
	reply.ReplyToMessageID = msg.MessageID
	h.outCh <- reply
	return nil
}

func (h *commandHandler) handleList(msg *tgbotapi.Message, lgr *slog.Logger) error {
	list, err := h.transmissionClient.TorrentGetAll(context.TODO())
	if err != nil {
		return fmt.Errorf("list failed: %w", err)
	}
	finished := make([]string, 0)
	inprogress := make([]string, 0)
	for _, item := range list {
		switch *item.Status {
		case transmissionrpc.TorrentStatusStopped, transmissionrpc.TorrentStatusSeed, transmissionrpc.TorrentStatusSeedWait:
			finishedText := fmt.Sprintf("ID: %d; Name: %s; Size: %s", item.ID, *item.Name, item.TotalSize)
			finished = append(finished, finishedText)
		default:
			unfinishedText := fmt.Sprintf("ID: %d; Name: %s; Percent Complete: %.2f%%; ETA: %s (status: %s)", item.ID, *item.Name, *item.PercentComplete*100, time.Duration(*item.ETA)*time.Second, item.Status)
			inprogress = append(inprogress, unfinishedText)
		}
	}

	finishedFull := append([]string{"Finished downloads:"}, finished...)
	finishedFullText := strings.Join(finishedFull, "\n\n")
	finishedMsg := tgbotapi.NewMessage(msg.Chat.ID, finishedFullText)
	h.outCh <- finishedMsg

	unfinishedFull := append([]string{"In Progress downloads:"}, inprogress...)
	unfinishedFullText := strings.Join(unfinishedFull, "\n\n")
	unfinishedMsg := tgbotapi.NewMessage(msg.Chat.ID, unfinishedFullText)
	h.outCh <- unfinishedMsg

	return nil
}
