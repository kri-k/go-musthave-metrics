package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/kri-k/go-musthave-metrics/internal/handler"
	models "github.com/kri-k/go-musthave-metrics/internal/model"
	"github.com/kri-k/go-musthave-metrics/internal/repository"
	"github.com/kri-k/go-musthave-metrics/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRouter() (http.Handler, repository.Repository) {
	storage := repository.NewMemStorage()
	svc := service.NewMetricsService(storage)
	h := handler.NewMetricsHandler(svc)

	r := chi.NewRouter()
	r.Get("/", h.Index)
	r.Post("/update", h.UpdateJSON)
	r.Post("/update/{type}/{name}/{value}", h.Update)
	r.Post("/value", h.ValueJSON)
	r.Get("/value/{type}/{name}", h.Value)

	return r, storage
}

func TestUpdate_GaugeSuccess(t *testing.T) {
	router, storage := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/testGauge/123.45", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	v, ok := storage.GetGauge("testGauge")
	assert.True(t, ok)
	assert.Equal(t, 123.45, v)
}

func TestUpdate_CounterSuccess(t *testing.T) {
	router, storage := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/update/counter/testCounter/527", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	v, ok := storage.GetCounter("testCounter")
	assert.True(t, ok)
	assert.Equal(t, int64(527), v)
}

func TestUpdate_InvalidType(t *testing.T) {
	router, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/update/unknown/test/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpdate_InvalidGaugeValue(t *testing.T) {
	router, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/test/notafloat", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpdate_InvalidCounterValue(t *testing.T) {
	router, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/update/counter/test/notanint", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpdate_InvalidPath(t *testing.T) {
	router, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdate_MethodNotAllowed(t *testing.T) {
	router, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/update/gauge/test/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestValue_GaugeSuccess(t *testing.T) {
	router, storage := newTestRouter()

	storage.UpdateGauge("testGauge", 123.45)

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/testGauge", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	v := rec.Body.String()
	assert.Equal(t, "123.45", v)
}

func TestUpdateJSON_GaugeSuccess(t *testing.T) {
	router, storage := newTestRouter()

	value := 1744184459.0
	body, err := json.Marshal(models.Metrics{
		ID:    "LastGC",
		MType: models.Gauge,
		Value: &value,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	v, ok := storage.GetGauge("LastGC")
	assert.True(t, ok)
	assert.Equal(t, value, v)

	var resp models.Metrics
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "LastGC", resp.ID)
	assert.Equal(t, models.Gauge, resp.MType)
	require.NotNil(t, resp.Value)
	assert.Equal(t, value, *resp.Value)
}

func TestUpdateJSON_CounterSuccess(t *testing.T) {
	router, storage := newTestRouter()

	delta := int64(10)
	body, err := json.Marshal(models.Metrics{
		ID:    "PollCount",
		MType: models.Counter,
		Delta: &delta,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	v, ok := storage.GetCounter("PollCount")
	assert.True(t, ok)
	assert.Equal(t, int64(10), v)

	var resp models.Metrics
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.NotNil(t, resp.Delta)
	assert.Equal(t, int64(10), *resp.Delta)
}

func TestValueJSON_GaugeSuccess(t *testing.T) {
	router, storage := newTestRouter()

	storage.UpdateGauge("LastGC", 1744184459)

	body, err := json.Marshal(models.Metrics{
		ID:    "LastGC",
		MType: models.Gauge,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp models.Metrics
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "LastGC", resp.ID)
	assert.Equal(t, models.Gauge, resp.MType)
	require.NotNil(t, resp.Value)
	assert.Equal(t, 1744184459.0, *resp.Value)
}

func TestValueJSON_NotFound(t *testing.T) {
	router, _ := newTestRouter()

	body, err := json.Marshal(models.Metrics{
		ID:    "Missing",
		MType: models.Gauge,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
