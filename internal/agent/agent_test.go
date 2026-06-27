package agent_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kri-k/go-musthave-metrics/internal/agent"
)

func TestPoll_IncrementsPollCount(t *testing.T) {
	a := agent.NewAgent()

	a.Poll()
	v1, ok := a.GetCounter("PollCount")
	if !ok || v1 != 1 {
		t.Errorf("expected PollCount=1, got %d (ok=%v)", v1, ok)
	}

	a.Poll()
	v2, ok := a.GetCounter("PollCount")
	if !ok || v2 != 2 {
		t.Errorf("expected PollCount=2, got %d (ok=%v)", v2, ok)
	}
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
		if _, ok := a.GetGauge(name); !ok {
			t.Errorf("expected gauge %s to be collected", name)
		}
	}
}

func TestPoll_UpdatesRandomValue(t *testing.T) {
	a := agent.NewAgent()
	a.Poll()
	v1, _ := a.GetGauge("RandomValue")
	a.Poll()
	v2, _ := a.GetGauge("RandomValue")

	if v1 == v2 {
		t.Errorf("expected RandomValue to change between polls, got %f and %f", v1, v2)
	}
}

func TestReport_SendsMetricsToServer(t *testing.T) {
	var mu sync.Mutex
	received := make([]string, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "text/plain" {
			t.Errorf("expected Content-Type text/plain, got %s", ct)
		}
		mu.Lock()
		received = append(received, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := agent.NewAgent()
	a.SetServerAddr(server.URL)
	a.Poll()
	a.Report()

	mu.Lock()
	defer mu.Unlock()

	if len(received) == 0 {
		t.Fatal("expected metrics to be sent to server")
	}

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

	if !foundPollCount {
		t.Error("expected PollCount counter to be sent")
	}
	if !foundAlloc {
		t.Error("expected Alloc gauge to be sent")
	}
}
