package tgbotbase

import (
	"log"
	"regexp"
	"strings"

	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

type ServiceMsg struct {
	stopBot bool
}

type MessageDealer interface {
	init(chan<- tgbotapi.Chattable, chan<- ServiceMsg)
	accept(tgbotapi.Message)
	run()
	name() string
}

type HandlerTrigger struct {
	re   *regexp.Regexp
	cmds map[string]bool
}

func NewHandlerTrigger(re *regexp.Regexp, cmds []string) HandlerTrigger {
	cmdmap := make(map[string]bool, len(cmds))
	for _, c := range cmds {
		cmdmap[c] = true
	}

	return HandlerTrigger{re: re,
		cmds: cmdmap}
}

func (t *HandlerTrigger) canHandle(msg tgbotapi.Message) bool {
	text := strings.ToLower(msg.Text)
	if t.re != nil && t.re.MatchString(text) {
		log.Printf("Message text '%s' matched regexp '%s'", msg.Text, t.re)
		return true
	}
	if msg.IsCommand() {
		cmd := msg.Command()
		if _, found := t.cmds[cmd]; found {
			log.Printf("Message text '%s' matched command '%s'", msg.Text, cmd)
			return true
		}
	}
	log.Printf("Message text '%s' doesn't match either commands '%v' or regexp '%s'", msg.Text, t.cmds, t.re)
	return false
}

type IncomingMessageHandler interface {
	Init(chan<- tgbotapi.Chattable, chan<- ServiceMsg) HandlerTrigger
	HandleOne(tgbotapi.Message)
	Name() string
}

type IncomingMessageDealer struct {
	handler IncomingMessageHandler
	trigger HandlerTrigger
	inMsgCh chan tgbotapi.Message
}

func NewIncomingMessageDealer(h IncomingMessageHandler) *IncomingMessageDealer {
	d := &IncomingMessageDealer{handler: h}
	return d
}

func (d *IncomingMessageDealer) init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- ServiceMsg) {
	d.trigger = d.handler.Init(outMsgCh, srvCh)
	d.inMsgCh = make(chan tgbotapi.Message, 0)
}

func (d *IncomingMessageDealer) accept(msg tgbotapi.Message) {
	if d.trigger.canHandle(msg) {
		d.inMsgCh <- msg
	}
}

func (d *IncomingMessageDealer) run() {
	go func() {
		for msg := range d.inMsgCh {
			d.handler.HandleOne(msg)
		}
	}()
}

func (d *IncomingMessageDealer) name() string {
	return d.handler.Name()
}

type BaseHandler struct {
	OutMsgCh chan<- tgbotapi.Chattable
	SrvCh    chan<- ServiceMsg
}

type BackgroundMessageHandler interface {
	Init(chan<- tgbotapi.Chattable, chan<- ServiceMsg)
	Run()
	Name() string
}

type BackgroundMessageDealer struct {
	h BackgroundMessageHandler
}

func NewBackgroundMessageDealer(h BackgroundMessageHandler) MessageDealer {
	return &BackgroundMessageDealer{h: h}
}

func (d *BackgroundMessageDealer) init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- ServiceMsg) {
	d.h.Init(outMsgCh, srvCh)
}

func (d *BackgroundMessageDealer) accept(tgbotapi.Message) {
	// doing nothing
}

func (d *BackgroundMessageDealer) run() {
	d.h.Run()
}

func (d *BackgroundMessageDealer) name() string {
	return d.h.Name()
}

type EngagementHandler interface {
	Name() string
	Engaged(chat *tgbotapi.Chat, user *tgbotapi.User)
	Disengaged(chat *tgbotapi.Chat, user *tgbotapi.User)
}

type EngagementMessageDealer struct {
	h EngagementHandler
}

func NewEngagementMessageDealer(h EngagementHandler) MessageDealer {
	return &EngagementMessageDealer{h: h}
}

func (d *EngagementMessageDealer) init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- ServiceMsg) {

}

func (d *EngagementMessageDealer) accept(msg tgbotapi.Message) {
	if msg.NewChatMembers != nil {
		for _, m := range *msg.NewChatMembers {
			if m.IsBot && m.UserName == thisBotUserName() {
				d.h.Engaged(msg.Chat, msg.From)
			}
		}
	}
	if msg.LeftChatMember != nil {
		if msg.LeftChatMember.UserName == thisBotUserName() {
			d.h.Disengaged(msg.Chat, msg.From)
		}
	}
}

func (d *EngagementMessageDealer) run() {

}

func (d *EngagementMessageDealer) name() string {
	return d.h.Name()
}
