package main

import (
	"log"
	"support_bot/services"
	"support_bot/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	services.Run(cfg)
}