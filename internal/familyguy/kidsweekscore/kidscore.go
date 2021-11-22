package kidsweekscore

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/ilyalavrinov/tgbots/pkg/tgbotbase"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

type kidScoreHandler struct {
	tgbotbase.BaseHandler

	storage Storage
}

func NewKidScoreHandler(storage Storage) tgbotbase.IncomingMessageHandler {
	return &kidScoreHandler{
		storage: storage,
	}
}

var _ tgbotbase.IncomingMessageHandler = &kidScoreHandler{}

func (h *kidScoreHandler) Name() string {
	return "Kids week score"
}

func (h *kidScoreHandler) HandleOne(msg tgbotapi.Message) {
	ctx := context.TODO()
	settings, err := h.storage.loadSettings(ctx, msg.Chat.ID)
	if err != nil {
		log.WithField("err", err).Error("Cannot load settings")
		return
	}

	isParent := false
	for _, p := range settings.parents {
		if p == strconv.Itoa(msg.From.ID) {
			isParent = true
			break
		}
	}

	if !isParent {
		log.WithFields(log.Fields{"chat": msg.Chat.ID, "id": msg.From.ID, "name": msg.From.UserName}).Debug("User is not a parent")
		return
	}

	var targetChild string
	for name, aliases := range settings.kidsAliases {
		for _, a := range aliases {
			log.WithFields(log.Fields{"alias": a}).Debug("Comparing text with child alias")
			if strings.Contains(strings.ToLower(msg.Text), strings.ToLower(a)) {
				targetChild = name
				break
			}
		}
	}
	if targetChild == "" {
		log.Debug("Target child not found")
		return
	}

	m := Unknown
	if strings.Contains(msg.Text, "+1") {
		m = Good
	} else if strings.Contains(msg.Text, "-1") {
		m = Bad
	}

	if m == Unknown {
		log.Error("Incoming text doesn't contain correct score marks")
		return
	}

	err = h.storage.add(ctx, msg.Chat.ID, targetChild, msg.Time(), m)
	if err != nil {
		log.WithFields(log.Fields{"err": err, "child": targetChild, "string": m}).Error("Cannot add score")
		return
	}

	replyText := "Принято!"
	positives, negatives, err := scoresThisWeek(ctx, h.storage, msg.Chat.ID, targetChild)
	if err != nil {
		log.WithFields(log.Fields{"err": err, "kid": targetChild}).Error("Cannot get this week scores")
	}

	replyText = fmt.Sprintf("%s Сейчас %d плюсов и %d минусов", replyText, positives, negatives)
	replyMsg := tgbotapi.NewMessage(msg.Chat.ID, replyText)
	replyMsg.BaseChat.ReplyToMessageID = msg.MessageID
	h.OutMsgCh <- replyMsg
}

func (h *kidScoreHandler) Init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- tgbotbase.ServiceMsg) tgbotbase.HandlerTrigger {
	h.OutMsgCh = outMsgCh

	return tgbotbase.NewHandlerTrigger(regexp.MustCompile("[\\+\\-]1"), nil)
}
