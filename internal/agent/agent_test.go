package agent_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kri-k/go-musthave-metrics/internal/agent"
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
	received := make([]string, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		mu.Lock()
		received = append(received, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
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
	for _, path := range received {
		if path == "/update/counter/PollCount/1" {
			foundPollCount = true
		}
		if strings.HasPrefix(path, "/update/gauge/Alloc/") {
			foundAlloc = true
		}
	}

	assert.True(t, foundPollCount, "expected PollCount counter to be sent")
	assert.True(t, foundAlloc, "expected Alloc gauge to be sent")
}
