package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/kri-k/go-musthave-metrics/internal/service"
)

type MetricsHandler struct {
	service *service.MetricsService
}

func NewMetricsHandler(s *service.MetricsService) *MetricsHandler {
	return &MetricsHandler{service: s}
}

func (h *MetricsHandler) Index(w http.ResponseWriter, r *http.Request) {
	stringBuilder := strings.Builder{}
	for _, m := range h.service.GetAllMetrics() {
		stringBuilder.WriteString(m.MType)
		stringBuilder.WriteRune(' ')
		stringBuilder.WriteString(m.ID)
		stringBuilder.WriteRune(' ')
		stringBuilder.WriteString(m.Value)
		stringBuilder.WriteRune('\n')
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(stringBuilder.String()))
}

func (h *MetricsHandler) Value(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "type")
	name := chi.URLParam(r, "name")

	value, err := h.service.GetMetric(mType, name)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(value))
}

func (h *MetricsHandler) Update(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "type")
	name := chi.URLParam(r, "name")
	value := chi.URLParam(r, "value")

	if err := h.service.UpdateMetric(mType, name, value); err != nil {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if updatedValue, err := h.service.GetMetric(mType, name); err == nil {
		w.Write([]byte(updatedValue))
	} else {
		log.Printf("error getting metric: %v", err)
	}
}
