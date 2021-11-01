package tgbotbase

import (
	"log"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

var botUserName string

func thisBotUserName() string {
	if botUserName == "" {
		panic("bot username not yet initialized")
	}
	return botUserName
}

type Bot struct {
	dealers []MessageDealer
	cfg     Config

	bot         *tgbotapi.BotAPI
	botChannels struct {
		in_msg_chan  tgbotapi.UpdatesChannel
		out_msg_chan chan tgbotapi.Chattable
		service_chan chan ServiceMsg
	}
}

func NewBot(cfg Config) *Bot {
	b := &Bot{dealers: make([]MessageDealer, 0),
		cfg: cfg}

	botToken := cfg.TGBot.Token
	log.Printf("Setting up a bot with token: %s", botToken)

	b.botChannels.out_msg_chan = make(chan tgbotapi.Chattable, 0)
	b.botChannels.service_chan = make(chan ServiceMsg, 0)

	if cfg.TGBot.SkipConnect {
		return b
	}

	// connecting to Telegram
	if cfg.Proxy_SOCKS5.Server != "" {
		log.Printf("Proxy is set, connecting to '%s' with credentials '%s':'%s'", cfg.Proxy_SOCKS5.Server, cfg.Proxy_SOCKS5.User, cfg.Proxy_SOCKS5.Pass)
		auth := proxy.Auth{User: cfg.Proxy_SOCKS5.User,
			Password: cfg.Proxy_SOCKS5.Pass}
		dialer, err := proxy.SOCKS5("tcp", cfg.Proxy_SOCKS5.Server, &auth, proxy.Direct)
		if err != nil {
			log.Panicf("Could get proxy dialer, error: %s", err)
		}
		httpTransport := &http.Transport{}
		httpTransport.Dial = dialer.Dial
		httpClient := &http.Client{Transport: httpTransport}
		b.bot, err = tgbotapi.NewBotAPIWithClient(botToken, httpClient)
		if err != nil {
			log.Panicf("Could not connect via proxy, error: %s", err)
		}
	} else {
		log.Printf("No proxy is set, going without any proxy")
		var err error
		b.bot, err = tgbotapi.NewBotAPI(botToken)
		if err != nil {
			log.Panicf("Could not connect directly, error: %s", err)
		}
	}

	botUserName = b.bot.Self.UserName
	log.Printf("Authorized on account %s", botUserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := b.bot.GetUpdatesChan(u)
	if err != nil {
		log.Panic(err)
	}
	b.botChannels.in_msg_chan = updates

	return b
}

func (b *Bot) AddHandler(d MessageDealer) {
	log.Printf("Preparing '%s' handler", d.name())
	d.init(b.botChannels.out_msg_chan, b.botChannels.service_chan)
	b.dealers = append(b.dealers, d)
}

func (b *Bot) Start() {
	log.Printf("Starting bot")
	for _, d := range b.dealers {
		log.Printf("Starting handler '%s'", d.name())
		d.run()
	}

	go b.serveReplies()
	isRunning := true
	for isRunning {
		select {
		case update := <-b.botChannels.in_msg_chan:
			log.Printf("Received an update from tgbotapi")
			if b.cfg.TGBot.Verbose {
				dumpUpdate(update)
			}
			if update.Message == nil {
				log.Print("Message: empty. Skipping")
				continue
			}

			for _, d := range b.dealers {
				d.accept(*update.Message)
			}
		case srvMsg := <-b.botChannels.service_chan:
			log.Printf("Received service message: %+v", srvMsg)
			continue
		}
	}
	time.Sleep(1 * time.Second)
	close(b.botChannels.out_msg_chan)

	log.Print("Main cycle has been aborted")
}

func (b *Bot) Send(msg tgbotapi.Chattable) {
	b.botChannels.out_msg_chan <- msg
}

func (b *Bot) serveReplies() {
	log.Print("Started serving replies")
	msg, notClosed := <-b.botChannels.out_msg_chan
	for ; notClosed; msg, notClosed = <-b.botChannels.out_msg_chan {
		log.Printf("Will send a reply")
		_, err := b.bot.Send(msg)
		if err != nil {
			log.Printf("Could not sent reply %+v due to error: %s", msg, err)
		}
	}

	log.Print("Finished serving replies")
}

func dumpUpdate(update tgbotapi.Update) {
	log.Printf("Update: %+v", update)
	if update.Message != nil {
		log.Printf("Message from: %s; Text: %s", update.Message.From.UserName, update.Message.Text)
		log.Printf("Message: %+v", update.Message)
		log.Printf("Message.Chat: %+v", update.Message.Chat)
		log.Printf("Message.NewChatMembers: %+v", update.Message.NewChatMembers)
	}
}
