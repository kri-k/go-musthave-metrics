package handler

import (
	"net/http"
	"strings"

	"github.com/kri-k/go-musthave-metrics/internal/service"
)

type MetricsHandler struct {
	service *service.MetricsService
}

func NewMetricsHandler(s *service.MetricsService) *MetricsHandler {
	return &MetricsHandler{service: s}
}

func (h *MetricsHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "update" {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	mType, name, value := parts[1], parts[2], parts[3]
	if name == "" || value == "" {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateMetric(mType, name, value); err != nil {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
}
