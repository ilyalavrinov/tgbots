package main

import (
	"log"
	"math/rand"
	"time"

	"github.com/ilyalavrinov/tgbots/internal/familyguy"
)

const cfg_filename = "familyguy.cfg"

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	log.Print("Starting my bot")

	err := familyguy.Start(cfg_filename)
	if err != nil {
		log.Printf("My bot could not be started due to error: %s", err)
	}

	log.Print("My bot has stopped working")
}
