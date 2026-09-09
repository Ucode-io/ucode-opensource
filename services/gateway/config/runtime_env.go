package config

import (
	"log"

	"github.com/joho/godotenv"
)

func loadRuntimeEnvironment() {
	loaded := false
	for _, path := range []string{"/app/.env", "/meta-ads.env", ".env"} {
		if err := godotenv.Load(path); err == nil {
			loaded = true
		}
	}

	if !loaded {
		log.Println("No .env file found")
	}
}
