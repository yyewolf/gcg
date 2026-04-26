package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yyewolf/gcg/internal/server"
)

//go:embed web/dist/*
var webAssets embed.FS

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	staticFS, err := fs.Sub(webAssets, "web/dist")
	if err != nil {
		log.Fatalf("load embedded assets: %v", err)
	}

	server := server.New(":8080", staticFS)
	if err := server.Run(ctx); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
