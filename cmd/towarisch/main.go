package main

import (
	"log"
	"math/rand"
	"time"

	"github.com/ilyalavrinov/tgbot-betterthanpbelov/mybot"
)

const cfg_filename = "mybot.cfg"

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	log.Print("Starting my bot")

	err := mybot.Start(cfg_filename)
	if err != nil {
		log.Printf("My bot could not be started due to error: %s", err)
	}

	log.Print("My bot has stopped working")
}
