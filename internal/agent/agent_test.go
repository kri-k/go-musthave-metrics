package agent_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/kri-k/go-musthave-metrics/internal/agent"
	models "github.com/kri-k/go-musthave-metrics/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoll_IncrementsPollCount(t *testing.T) {
	a := agent.NewAgent()

	a.Poll()
	v1, ok := a.GetCounterForTest("PollCount")
	assert.True(t, ok)
	assert.Equal(t, int64(1), v1)

	a.Poll()
	v2, ok := a.GetCounterForTest("PollCount")
	assert.True(t, ok)
	assert.Equal(t, int64(2), v2)
}

func TestPoll_CollectsRuntimeMetrics(t *testing.T) {
	a := agent.NewAgent()
	a.Poll()

	expectedGauges := []string{
		"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys",
		"HeapAlloc", "HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased",
		"HeapSys", "LastGC", "Lookups", "MCacheInuse", "MCacheSys",
		"MSpanInuse", "MSpanSys", "Mallocs", "NextGC", "NumForcedGC",
		"NumGC", "OtherSys", "PauseTotalNs", "StackInuse", "StackSys",
		"Sys", "TotalAlloc", "RandomValue",
	}

	for _, name := range expectedGauges {
		if _, ok := a.GetGaugeForTest(name); !ok {
			assert.True(t, ok, "expected gauge %s to be collected", name)
		}
	}
}

func TestPoll_UpdatesRandomValue(t *testing.T) {
	a := agent.NewAgent()
	a.Poll()
	v1, _ := a.GetGaugeForTest("RandomValue")
	a.Poll()
	v2, _ := a.GetGaugeForTest("RandomValue")

	assert.NotEqual(t, v1, v2, "expected RandomValue to change between the polls")
}

func TestReport_SendsMetricsToServer(t *testing.T) {
	var mu sync.Mutex
	received := make([]models.Metrics, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		assert.Equal(t, "/update", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var m models.Metrics
		require.NoError(t, json.Unmarshal(body, &m))

		mu.Lock()
		received = append(received, m)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(m)
	}))
	defer server.Close()

	a := agent.NewAgentWithConfig(server.URL, 0, 0)
	a.Poll()
	a.Report()

	mu.Lock()
	defer mu.Unlock()

	require.NotEmpty(t, received, "expected metrics to be sent to server")

	foundPollCount := false
	foundAlloc := false
	for _, m := range received {
		if m.ID == "PollCount" && m.MType == models.Counter {
			require.NotNil(t, m.Delta)
			assert.Equal(t, int64(1), *m.Delta)
			foundPollCount = true
		}
		if m.ID == "Alloc" && m.MType == models.Gauge {
			require.NotNil(t, m.Value)
			foundAlloc = true
		}
	}

	assert.True(t, foundPollCount, "expected PollCount counter to be sent")
	assert.True(t, foundAlloc, "expected Alloc gauge to be sent")
}
