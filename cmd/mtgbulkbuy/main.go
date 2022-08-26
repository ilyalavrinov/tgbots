package main

import (
	"log"

	"github.com/ilyalavrinov/tgbots/internal/mtgbulkbuy"
)

const cfg_filename = "mtgbulkbuy.cfg"

func main() {

	log.Print("Starting my bot")

	err := mtgbulkbuy.Start(cfg_filename)
	if err != nil {
		log.Printf("My bot could not be started due to error: %s", err)
	}

	log.Print("My bot has stopped working")
}
