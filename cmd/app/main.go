package main

import (
	"log"

	"pellets-tracker/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("pellets tracker starting on %s (tsnet=%v)", cfg.ListenAddr, cfg.TsnetEnabled)
	// TODO: initialize store, http server, tsnet integration, etc.
}
