package familyguy

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/ilyalavrinov/tgbots/internal/familyguy/kidsweekscore"
	"github.com/ilyalavrinov/tgbots/pkg/tgbotbase"
	"gopkg.in/gcfg.v1"
)

type Config struct {
	tgbotbase.Config
	Redis tgbotbase.RedisConfig
}

func NewConfig(filename string) (Config, error) {
	log.Printf("Reading configuration from: %s", filename)

	var cfg Config

	err := gcfg.ReadFileInto(&cfg, filename)
	if err != nil {
		log.Printf("Could not correctly parse configuration file: %s; error: %s", filename, err)
		return cfg, err
	}

	log.Printf("Configuration has been successfully read from %s: %+v", filename, cfg)
	return cfg, nil
}

func Start(cfg_filename string) error {
	log.SetLevel(log.DebugLevel)
	log.Print("Starting my bot")

	fullcfg, err := NewConfig(cfg_filename)
	if err != nil {
		log.Printf("My bot cannot be sarted due to error: %s", err)
		return err
	}

	log.Printf("Starting bot with full config: %+v", fullcfg)

	tgcfg := tgbotbase.Config{TGBot: fullcfg.TGBot,
		Proxy_SOCKS5: fullcfg.Proxy_SOCKS5}
	bot := tgbotbase.NewBot(tgcfg)

	rediscfg := fullcfg.Redis
	redispool := tgbotbase.NewRedisPool(context.TODO(), rediscfg)
	propstorage := tgbotbase.NewRedisPropertyStorage(redispool)
	kidstorage := kidsweekscore.NewRedisStorage(redispool)
	cron := tgbotbase.NewCron()

	bot.AddHandler(tgbotbase.NewIncomingMessageDealer(kidsweekscore.NewKidScoreHandler(kidstorage)))
	bot.AddHandler(tgbotbase.NewBackgroundMessageDealer(kidsweekscore.NewKidScoreResult(kidstorage, cron, propstorage)))
	bot.Start()

	log.Print("Stopping my bot")
	return nil
}
