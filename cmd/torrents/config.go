package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type config struct {
	allowedUsers         map[int64]bool
	token                string
	transmissionPassword string
}

func readConfig() (config, error) {
	users := os.Getenv("TGTORRENTSBOT_USERS")
	usersSeparated := strings.Split(users, ",")
	if len(users) == 0 {
		return config{}, fmt.Errorf("no allowed users found")
	}

	tgtoken := os.Getenv("TGTORRENTSBOT_TOKEN")
	if tgtoken == "" {
		return config{}, fmt.Errorf("no token found")
	}

	transmissionPWD := os.Getenv("TGTORRENTSBOT_TRANSMISSION_PASSWORD")
	if transmissionPWD == "" {
		return config{}, fmt.Errorf("transmission password not set")
	}

	allowedUsers := make(map[int64]bool, len(usersSeparated))
	for _, u := range usersSeparated {
		id, err := strconv.ParseInt(u, 10, 64)
		if err != nil {
			return config{}, fmt.Errorf("cannot convert user %s to int64 id: %w", u, err)
		}
		allowedUsers[id] = true
	}

	return config{
		allowedUsers:         allowedUsers,
		token:                tgtoken,
		transmissionPassword: transmissionPWD,
	}, nil
}
