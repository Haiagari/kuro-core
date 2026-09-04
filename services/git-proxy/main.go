package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"kuro/git-proxy/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
