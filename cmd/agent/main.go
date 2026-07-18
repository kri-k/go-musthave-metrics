package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kri-k/go-musthave-metrics/internal/agent"
)

var (
	addr           = flag.String("a", "localhost:8080", "address and port to run server")
	reportInterval = flag.Int("r", 10, "frequency of sending metrics to the server")
	pollInterval   = flag.Int("p", 2, "frequency of polling metrics from the runtime package")
)

func main() {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := agent.NewAgentWithConfig(*addr, *pollInterval, *reportInterval)
	a.Run(ctx)
	log.Println("agent stopped")
}
