package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/kri-k/go-musthave-metrics/internal/agent"
	"github.com/kri-k/go-musthave-metrics/internal/logger"
	"github.com/kri-k/go-musthave-metrics/internal/util"
)

var (
	flagAddr           = flag.String("a", "localhost:8080", "address and port to run server")
	flagReportInterval = flag.Int("r", 10, "frequency of sending metrics to the server")
	flagPollInterval   = flag.Int("p", 2, "frequency of polling metrics from the runtime package")
	flagRateLimit      = flag.Int("l", 1, "maximum number of concurrent requests to the server")
	flagKey            = flag.String("k", "", "key for HMAC-SHA256 signatures")
)

func main() {
	logger.Initialize("INFO")
	defer logger.Log.Sync()

	flag.Parse()

	addr := util.GetEnvOrDefaultString("ADDRESS", *flagAddr)

	reportInterval, err := util.GetEnvOrDefault("REPORT_INTERVAL", *flagReportInterval, strconv.Atoi)
	if err != nil {
		logger.Sugar.Fatalln(err.Error())
	}

	pollInterval, err := util.GetEnvOrDefault("POLL_INTERVAL", *flagPollInterval, strconv.Atoi)
	if err != nil {
		logger.Sugar.Fatalln(err.Error())
	}

	rateLimit, err := util.GetEnvOrDefault("RATE_LIMIT", *flagRateLimit, strconv.Atoi)
	if err != nil {
		logger.Sugar.Fatalln(err.Error())
	}
	if rateLimit <= 0 {
		logger.Sugar.Fatalln("RATE_LIMIT / -l must be greater than zero")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := agent.NewAgentWithRateLimit(addr, pollInterval, reportInterval, rateLimit, util.GetEnvOrDefaultString("KEY", *flagKey))
	a.Run(ctx)
	logger.Log.Info("agent stopped")
}
