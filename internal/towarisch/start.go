package mybot

import (
	log "github.com/sirupsen/logrus"

	"github.com/admirallarimda/tgbotbase"
	cmd "github.com/ilyalavrinov/tgbot-betterthanpbelov/mybot/commandhandler"
)

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
	redispool := tgbotbase.NewRedisPool(rediscfg)
	propstorage := tgbotbase.NewRedisPropertyStorage(redispool)
	remindstorage := cmd.NewRedisReminderStorage(redispool)

	cron := tgbotbase.NewCron()

	bot.AddHandler(tgbotbase.NewIncomingMessageDealer(cmd.NewPropertyHandler(propstorage)))
	bot.AddHandler(tgbotbase.NewIncomingMessageDealer(cmd.NewWeatherHandler(fullcfg.Weather.Token, redispool, propstorage)))
	bot.AddHandler(tgbotbase.NewIncomingMessageDealer(cmd.NewRemindHandler(cron, remindstorage, propstorage)))
	bot.AddHandler(tgbotbase.NewBackgroundMessageDealer(cmd.NewKittiesHandler(cron, propstorage)))
	bot.AddHandler(tgbotbase.NewBackgroundMessageDealer(cmd.NewWeatherMorningHandler(cron, propstorage, redispool, fullcfg.Weather.Token)))
	bot.AddHandler(tgbotbase.NewBackgroundMessageDealer(cmd.NewCovid19Handler(cron, propstorage)))
	bot.AddHandler(tgbotbase.NewBackgroundMessageDealer(cmd.NewNewsNNHandler(cron, propstorage)))
	bot.Start()

	log.Print("Stopping my bot")
	return nil
}
