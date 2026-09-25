package handler_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/kri-k/go-musthave-metrics/internal/handler"
	"github.com/kri-k/go-musthave-metrics/internal/middleware"
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
	r.Use(middleware.Gzip)
	r.Use(chiMiddleware.StripSlashes)
	r.Get("/", h.Index)
	r.Post("/update", h.UpdateJSON)
	r.Post("/update/{type}/{name}/{value}", h.Update)
	r.Post("/updates", h.UpdatesJSON)
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

func TestUpdateJSON_GzipRequest(t *testing.T) {
	router, storage := newTestRouter()

	value := 42.5
	payload, err := json.Marshal(models.Metrics{
		ID:    "Alloc",
		MType: models.Gauge,
		Value: &value,
	})
	require.NoError(t, err)

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err = zw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	req := httptest.NewRequest(http.MethodPost, "/update", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	v, ok := storage.GetGauge("Alloc")
	assert.True(t, ok)
	assert.Equal(t, value, v)
}

func TestValueJSON_GzipResponse(t *testing.T) {
	router, storage := newTestRouter()
	storage.UpdateGauge("Alloc", 100)

	body, err := json.Marshal(models.Metrics{
		ID:    "Alloc",
		MType: models.Gauge,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	gr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	defer gr.Close()

	decoded, err := io.ReadAll(gr)
	require.NoError(t, err)

	var resp models.Metrics
	require.NoError(t, json.Unmarshal(decoded, &resp))
	require.NotNil(t, resp.Value)
	assert.Equal(t, 100.0, *resp.Value)
}

func TestUpdatesJSON_BatchSuccess(t *testing.T) {
	router, storage := newTestRouter()

	gaugeValue := 42.5
	counterDelta := int64(7)
	body, err := json.Marshal([]models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &gaugeValue},
		{ID: "PollCount", MType: models.Counter, Delta: &counterDelta},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	v, ok := storage.GetGauge("Alloc")
	assert.True(t, ok)
	assert.Equal(t, gaugeValue, v)

	c, ok := storage.GetCounter("PollCount")
	assert.True(t, ok)
	assert.Equal(t, int64(7), c)
}

func TestUpdatesJSON_DuplicateCounter(t *testing.T) {
	router, storage := newTestRouter()

	first := int64(3)
	second := int64(5)
	body, err := json.Marshal([]models.Metrics{
		{ID: "PollCount", MType: models.Counter, Delta: &first},
		{ID: "PollCount", MType: models.Counter, Delta: &second},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/updates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	v, ok := storage.GetCounter("PollCount")
	assert.True(t, ok)
	assert.Equal(t, int64(8), v)
}

func TestUpdatesJSON_InvalidType(t *testing.T) {
	router, _ := newTestRouter()

	delta := int64(1)
	body, err := json.Marshal([]models.Metrics{
		{ID: "bad", MType: "unknown", Delta: &delta},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpdatesJSON_EmptyBatch(t *testing.T) {
	router, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader([]byte("[]")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdatesJSON_GzipRequest(t *testing.T) {
	router, storage := newTestRouter()

	value := 99.0
	payload, err := json.Marshal([]models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &value},
	})
	require.NoError(t, err)

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err = zw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	req := httptest.NewRequest(http.MethodPost, "/updates/", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	v, ok := storage.GetGauge("Alloc")
	assert.True(t, ok)
	assert.Equal(t, value, v)
}

func TestIndex_GzipHTMLResponse(t *testing.T) {
	router, storage := newTestRouter()
	storage.UpdateGauge("Alloc", 1)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html", rec.Header().Get("Content-Type"))
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

	gr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	defer gr.Close()

	body, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Alloc")
}

// Override batch writes to exercise the real handler and service with a failing storage.
type failingBatchRepository struct {
	repository.Repository
	calls int
}

func (r *failingBatchRepository) UpdateMetrics([]models.Metrics) error {
	r.calls++
	return fmt.Errorf("write batch: %w", fmt.Errorf("database unavailable"))
}

func TestUpdatesJSON_ErrorStatus(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status int
		calls  int
	}{
		{"storage failure", `[{"id":"Alloc","type":"gauge","value":1}]`, http.StatusInternalServerError, 1},
		{"missing id", `[{"type":"gauge","value":1}]`, http.StatusBadRequest, 0},
		{"missing value", `[{"id":"Alloc","type":"gauge"}]`, http.StatusBadRequest, 0},
		{"missing delta", `[{"id":"PollCount","type":"counter"}]`, http.StatusBadRequest, 0},
		{"unknown type", `[{"id":"Alloc","type":"unknown"}]`, http.StatusBadRequest, 0},
		{"invalid JSON", `[`, http.StatusBadRequest, 0},
		{"invalid later metric", `[{"id":"Alloc","type":"gauge","value":1},{"id":"PollCount","type":"counter"}]`, http.StatusBadRequest, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &failingBatchRepository{Repository: repository.NewMemStorage()}
			h := handler.NewMetricsHandler(service.NewMetricsService(repo))
			rec := httptest.NewRecorder()
			h.UpdatesJSON(rec, httptest.NewRequest(http.MethodPost, "/updates", strings.NewReader(tt.body)))
			require.Equal(t, tt.status, rec.Code, rec.Body.String())
			require.Equal(t, tt.calls, repo.calls)
		})
	}
}
