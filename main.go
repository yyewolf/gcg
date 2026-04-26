package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/yyewolf/gcg/internal/game"
	"github.com/yyewolf/gcg/internal/server"
)

//go:embed web/dist/*
var webAssets embed.FS

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config := game.DefaultMapConfig()
	if rawPlayerCount := os.Getenv("GCG_PLAYERS"); rawPlayerCount != "" {
		playerCount, err := strconv.Atoi(rawPlayerCount)
		if err != nil {
			log.Printf("invalid GCG_PLAYERS value %q, using default %d", rawPlayerCount, config.PlayerCount)
		} else {
			config.PlayerCount = playerCount
		}
	}

	staticFS, err := fs.Sub(webAssets, "web/dist")
	if err != nil {
		log.Fatalf("load embedded assets: %v", err)
	}

	server := server.New(":8080", staticFS, game.NewEngineWithConfig(config))
	if err := server.Run(ctx); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
