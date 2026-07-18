package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/kri-k/go-musthave-metrics/internal/agent"
	"github.com/kri-k/go-musthave-metrics/internal/util"
)

var (
	flagAddr           = flag.String("a", "localhost:8080", "address and port to run server")
	flagReportInterval = flag.Int("r", 10, "frequency of sending metrics to the server")
	flagPollInterval   = flag.Int("p", 2, "frequency of polling metrics from the runtime package")
)

func main() {
	flag.Parse()

	addr := util.GetEnvOrDefaultString("ADDRESS", *flagAddr)
	reportInterval := util.GetEnvOrDefault("REPORT_INTERVAL", *flagReportInterval, strconv.Atoi)
	pollInterval := util.GetEnvOrDefault("POLL_INTERVAL", *flagPollInterval, strconv.Atoi)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := agent.NewAgentWithConfig(addr, pollInterval, reportInterval)
	a.Run(ctx)
	log.Println("agent stopped")
}
