package main

import (
	"log"
	"net/http"

	"github.com/kri-k/go-musthave-metrics/internal/handler"
	"github.com/kri-k/go-musthave-metrics/internal/repository"
	"github.com/kri-k/go-musthave-metrics/internal/service"
)

func main() {
	storage := repository.NewMemStorage()
	svc := service.NewMetricsService(storage)
	h := handler.NewMetricsHandler(svc)

	http.HandleFunc("/update/", h.Update)

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
