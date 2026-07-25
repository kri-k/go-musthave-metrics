package main

import (
	"flag"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kri-k/go-musthave-metrics/internal/handler"
	"github.com/kri-k/go-musthave-metrics/internal/logger"
	"github.com/kri-k/go-musthave-metrics/internal/repository"
	"github.com/kri-k/go-musthave-metrics/internal/service"
	"github.com/kri-k/go-musthave-metrics/internal/util"
)

var flagAddr = flag.String("a", "localhost:8080", "address and port to run server")

func main() {
	logger.Initialize("INFO")
	defer logger.Log.Sync()

	flag.Parse()
	addr := util.GetEnvOrDefaultString("ADDRESS", *flagAddr)

	storage := repository.NewMemStorage()
	svc := service.NewMetricsService(storage)
	h := handler.NewMetricsHandler(svc)

	r := chi.NewRouter()
	r.Use(logger.WithLogging)
	r.Get("/", h.Index)
	r.Post("/update", h.UpdateJSON)
	r.Post("/update/{type}/{name}/{value}", h.Update)
	r.Post("/value", h.ValueJSON)
	r.Get("/value/{type}/{name}", h.Value)

	logger.Sugar.Info("Starting server on ", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		logger.Log.Fatal(err.Error())
	}
}
