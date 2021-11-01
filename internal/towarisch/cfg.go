package towarisch

import (
	"log"

	"github.com/ilyalavrinov/tgbots/pkg/tgbotbase"
	"gopkg.in/gcfg.v1"
)

type Config struct {
	tgbotbase.Config
	Redis   tgbotbase.RedisConfig
	Weather struct {
		Token string
	}

	Owners struct {
		ID []string
	}
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
