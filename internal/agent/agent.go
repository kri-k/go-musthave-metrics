package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math/rand"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/kri-k/go-musthave-metrics/internal/logger"
	models "github.com/kri-k/go-musthave-metrics/internal/model"
)

const (
	defaultPollInterval   = 2
	defaultReportInterval = 10
	defaultServerAddr     = "http://localhost:8080"
)

type Agent struct {
	serverAddr     string
	pollInterval   time.Duration
	reportInterval time.Duration
	client         *resty.Client

	mu       sync.Mutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewAgent() *Agent {
	return NewAgentWithConfig("", 0, 0)
}

func NewAgentWithConfig(serverAddr string, pollIntervalSec, reportIntervalSec int) *Agent {
	if serverAddr == "" {
		serverAddr = defaultServerAddr
	} else {
		serverAddr = normalizeServerAddr(serverAddr)
	}
	if pollIntervalSec <= 0 {
		pollIntervalSec = defaultPollInterval
	}
	if reportIntervalSec <= 0 {
		reportIntervalSec = defaultReportInterval
	}

	return &Agent{
		serverAddr:     serverAddr,
		pollInterval:   time.Duration(pollIntervalSec) * time.Second,
		reportInterval: time.Duration(reportIntervalSec) * time.Second,
		client:         resty.New(),
		gauges:         make(map[string]float64),
		counters:       make(map[string]int64),
	}
}

func normalizeServerAddr(addr string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	return "http://" + addr
}

func (a *Agent) Run(ctx context.Context) {
	logger.Sugar.Infow(
		"starting agent with settings",
		"pollInterval", a.pollInterval,
		"reportInterval", a.reportInterval,
		"serverAddr", a.serverAddr,
	)

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		a.pollLoop(ctx)
	}()
	go func() {
		defer wg.Done()
		a.reportLoop(ctx)
	}()

	wg.Wait()
}

func (a *Agent) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	a.Poll()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.Poll()
		}
	}
}

func (a *Agent) reportLoop(ctx context.Context) {
	ticker := time.NewTicker(a.reportInterval)
	defer ticker.Stop()

	a.Report()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.Report()
		}
	}
}

func (a *Agent) Poll() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	a.mu.Lock()
	defer a.mu.Unlock()

	logger.Log.Info("polling metrics...")
	a.gauges["Alloc"] = float64(memStats.Alloc)
	a.gauges["BuckHashSys"] = float64(memStats.BuckHashSys)
	a.gauges["Frees"] = float64(memStats.Frees)
	a.gauges["GCCPUFraction"] = memStats.GCCPUFraction
	a.gauges["GCSys"] = float64(memStats.GCSys)
	a.gauges["HeapAlloc"] = float64(memStats.HeapAlloc)
	a.gauges["HeapIdle"] = float64(memStats.HeapIdle)
	a.gauges["HeapInuse"] = float64(memStats.HeapInuse)
	a.gauges["HeapObjects"] = float64(memStats.HeapObjects)
	a.gauges["HeapReleased"] = float64(memStats.HeapReleased)
	a.gauges["HeapSys"] = float64(memStats.HeapSys)
	a.gauges["LastGC"] = float64(memStats.LastGC)
	a.gauges["Lookups"] = float64(memStats.Lookups)
	a.gauges["MCacheInuse"] = float64(memStats.MCacheInuse)
	a.gauges["MCacheSys"] = float64(memStats.MCacheSys)
	a.gauges["MSpanInuse"] = float64(memStats.MSpanInuse)
	a.gauges["MSpanSys"] = float64(memStats.MSpanSys)
	a.gauges["Mallocs"] = float64(memStats.Mallocs)
	a.gauges["NextGC"] = float64(memStats.NextGC)
	a.gauges["NumForcedGC"] = float64(memStats.NumForcedGC)
	a.gauges["NumGC"] = float64(memStats.NumGC)
	a.gauges["OtherSys"] = float64(memStats.OtherSys)
	a.gauges["PauseTotalNs"] = float64(memStats.PauseTotalNs)
	a.gauges["StackInuse"] = float64(memStats.StackInuse)
	a.gauges["StackSys"] = float64(memStats.StackSys)
	a.gauges["Sys"] = float64(memStats.Sys)
	a.gauges["TotalAlloc"] = float64(memStats.TotalAlloc)
	a.gauges["RandomValue"] = rand.Float64()

	a.counters["PollCount"]++
}

func (a *Agent) Report() {
	a.mu.Lock()
	gauges := make(map[string]float64, len(a.gauges))
	maps.Copy(gauges, a.gauges)
	counters := make(map[string]int64, len(a.counters))
	maps.Copy(counters, a.counters)
	a.counters["PollCount"] = 0
	a.mu.Unlock()

	metrics := make([]models.Metrics, 0, len(gauges)+len(counters))
	for name, value := range gauges {
		v := value
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Gauge,
			Value: &v,
		})
	}
	for name, value := range counters {
		d := value
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Counter,
			Delta: &d,
		})
	}

	if len(metrics) == 0 {
		return
	}

	logger.Log.Info("reporting metrics...")
	a.sendMetrics(metrics)
}

func (a *Agent) sendMetrics(metrics []models.Metrics) {
	body, err := compressJSON(metrics)
	if err != nil {
		logger.Sugar.Errorf("failed to compress metrics: %s", err)
		return
	}

	url := fmt.Sprintf("%s/updates/", a.serverAddr)
	r, err := a.client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Content-Encoding", "gzip").
		SetHeader("Accept-Encoding", "gzip").
		SetBody(body).
		Post(url)
	if err != nil {
		logger.Sugar.Errorf("failed to send metrics: %s", err)
	} else if r.StatusCode() != http.StatusOK {
		logger.Sugar.Errorf("failed to send metrics: response status %s", r.Status())
	}
}

func compressJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if err := json.NewEncoder(zw).Encode(v); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *Agent) GetGaugeForTest(name string) (float64, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	v, ok := a.gauges[name]
	return v, ok
}

func (a *Agent) GetCounterForTest(name string) (int64, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	v, ok := a.counters[name]
	return v, ok
}
