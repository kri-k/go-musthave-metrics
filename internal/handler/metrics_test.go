package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kri-k/go-musthave-metrics/internal/handler"
	"github.com/kri-k/go-musthave-metrics/internal/repository"
	"github.com/kri-k/go-musthave-metrics/internal/service"
)

func newTestHandler() (*handler.MetricsHandler, repository.Repository) {
	storage := repository.NewMemStorage()
	svc := service.NewMetricsService(storage)
	return handler.NewMetricsHandler(svc), storage
}

func TestUpdate_GaugeSuccess(t *testing.T) {
	h, storage := newTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/testGauge/123.45", nil)
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	v, ok := storage.GetGauge("testGauge")
	if !ok {
		t.Fatal("gauge not found in storage")
	}
	if v != 123.45 {
		t.Errorf("expected gauge value 123.45, got %f", v)
	}
}

func TestUpdate_CounterSuccess(t *testing.T) {
	h, storage := newTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/update/counter/testCounter/527", nil)
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	v, ok := storage.GetCounter("testCounter")
	if !ok {
		t.Fatal("counter not found in storage")
	}
	if v != 527 {
		t.Errorf("expected counter value 527, got %d", v)
	}
}

func TestUpdate_InvalidType(t *testing.T) {
	h, _ := newTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/update/unknown/test/1", nil)
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpdate_InvalidGaugeValue(t *testing.T) {
	h, _ := newTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/test/notafloat", nil)
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpdate_InvalidCounterValue(t *testing.T) {
	h, _ := newTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/update/counter/test/notanint", nil)
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpdate_InvalidPath(t *testing.T) {
	h, _ := newTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/test", nil)
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpdate_MethodNotAllowed(t *testing.T) {
	h, _ := newTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/update/gauge/test/1", nil)
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
