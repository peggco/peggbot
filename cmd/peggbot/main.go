package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/peggco/peggbot/internal/config"
	"github.com/peggco/peggbot/internal/github"
	"github.com/peggco/peggbot/internal/modes"
)

func main() {
	log.SetPrefix("peggbot: ")
	log.SetFlags(log.LstdFlags)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	gh, err := github.NewClient(cfg.Token, cfg.Owner, cfg.Repo, cfg.GHESBaseURL)
	if err != nil {
		log.Fatalf("github: %v", err)
	}

	if err := modes.Run(ctx, cfg, gh); err != nil {
		log.Printf("mode %s failed: %v", cfg.Mode, err)
		os.Exit(1)
	}
	log.Printf("mode %s finished successfully", cfg.Mode)
}
