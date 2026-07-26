package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	models "github.com/kri-k/go-musthave-metrics/internal/model"
	"github.com/kri-k/go-musthave-metrics/internal/service"
	"github.com/kri-k/go-musthave-metrics/internal/util"
)

type MetricsHandler struct {
	service *service.MetricsService
}

func NewMetricsHandler(s *service.MetricsService) *MetricsHandler {
	return &MetricsHandler{service: s}
}

func (h *MetricsHandler) Index(w http.ResponseWriter, _ *http.Request) {
	stringBuilder := strings.Builder{}
	stringBuilder.WriteString("<html><body>")
	for _, m := range h.service.GetAllMetrics() {
		stringBuilder.WriteString("<div>")
		stringBuilder.WriteString(m.MType)
		stringBuilder.WriteRune(' ')
		stringBuilder.WriteString(m.ID)
		stringBuilder.WriteRune(' ')
		stringBuilder.WriteString(metricValueString(m))
		stringBuilder.WriteString("</div>")
	}
	stringBuilder.WriteString("</body></html>")
	w.Header().Set("Content-Type", "text/html")
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

func (h *MetricsHandler) ValueJSON(w http.ResponseWriter, r *http.Request) {
	var m models.Metrics
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.service.GetMetricJSON(m.MType, m.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *MetricsHandler) Update(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "type")
	name := chi.URLParam(r, "name")
	value := chi.URLParam(r, "value")

	updatedValue, err := h.service.UpdateMetric(mType, name, value)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(updatedValue))
}

func (h *MetricsHandler) UpdateJSON(w http.ResponseWriter, r *http.Request) {
	var m models.Metrics
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updated, err := h.service.UpdateMetricJSON(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updated)
}

func metricValueString(m models.Metrics) string {
	switch m.MType {
	case models.Gauge:
		if m.Value != nil {
			return util.GaugeToString(*m.Value)
		}
	case models.Counter:
		if m.Delta != nil {
			return util.CounterToString(*m.Delta)
		}
	}
	return ""
}
