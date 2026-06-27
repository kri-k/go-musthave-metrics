package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kri-k/go-musthave-metrics/internal/agent"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := agent.NewAgent()
	a.Run(ctx)
	log.Println("agent stopped")
}
