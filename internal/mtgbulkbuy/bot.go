package mtgbulkbuy

import (
	"flag"

	"github.com/ilyalavrinov/tgbots/pkg/tgbotbase"
	"gopkg.in/gcfg.v1"
)

type config struct {
	tgbotbase.Config
}

func Start(cfgFilename string) error {
	flag.Parse()

	var cfg config

	if err := gcfg.ReadFileInto(&cfg, cfgFilename); err != nil {
		Fatalw("Cannot read config file",
			"filename", cfgFilename)
		return err
	}

	tgbot := tgbotbase.NewBot(tgbotbase.Config{TGBot: cfg.TGBot, Proxy_SOCKS5: cfg.Proxy_SOCKS5})
	tgbot.AddHandler(tgbotbase.NewIncomingMessageDealer(NewSearchHandler()))

	Info("Starting bot")
	tgbot.Start()
	Info("Stopping bot")
	return nil
}
