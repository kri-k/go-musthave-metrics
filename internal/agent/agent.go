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
	"github.com/kri-k/go-musthave-metrics/internal/retry"
	"github.com/kri-k/go-musthave-metrics/internal/signature"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
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
	key            string
	rateLimit      int

	mu       sync.Mutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewAgent() *Agent {
	return NewAgentWithConfig("", 0, 0)
}

func NewAgentWithConfig(serverAddr string, pollIntervalSec, reportIntervalSec int, key ...string) *Agent {
	var signingKey string
	if len(key) > 0 {
		signingKey = key[0]
	}
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
		key:            signingKey,
		rateLimit:      1,
		gauges:         make(map[string]float64),
		counters:       make(map[string]int64),
	}
}

func NewAgentWithRateLimit(serverAddr string, pollIntervalSec, reportIntervalSec, rateLimit int, key string) *Agent {
	a := NewAgentWithConfig(serverAddr, pollIntervalSec, reportIntervalSec, key)
	if rateLimit > 0 {
		a.rateLimit = rateLimit
	}
	return a
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
		"rateLimit", a.rateLimit,
	)

	var wg sync.WaitGroup

	jobs := make(chan []models.Metrics, a.rateLimit)
	wg.Add(3 + a.rateLimit)
	for i := 0; i < a.rateLimit; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case metrics, ok := <-jobs:
					if !ok {
						return
					}
					a.sendMetrics(ctx, metrics)
				}
			}
		}()
	}
	go func() {
		defer wg.Done()
		a.systemPollLoop(ctx)
	}()
	go func() {
		defer wg.Done()
		a.pollLoop(ctx)
	}()
	go func() {
		defer wg.Done()
		defer close(jobs)
		a.reportLoop(ctx, jobs)
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

func (a *Agent) reportLoop(ctx context.Context, jobs chan<- []models.Metrics) {
	ticker := time.NewTicker(a.reportInterval)
	defer ticker.Stop()
	for {
		metrics := a.snapshot()
		if len(metrics) > 0 {
			select {
			case <-ctx.Done():
				return
			case jobs <- metrics:
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *Agent) systemPollLoop(ctx context.Context) {
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()
	for {
		a.pollSystem(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *Agent) pollSystem(ctx context.Context) {
	memory, memoryErr := mem.VirtualMemoryWithContext(ctx)
	utilization, cpuErr := cpu.PercentWithContext(ctx, 0, true)
	if memoryErr != nil {
		logger.Sugar.Errorf("failed to collect memory metrics: %s", memoryErr)
	}
	if cpuErr != nil {
		logger.Sugar.Errorf("failed to collect CPU metrics: %s", cpuErr)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if memoryErr == nil {
		a.gauges["TotalMemory"] = float64(memory.Total)
		a.gauges["FreeMemory"] = float64(memory.Free)
	}
	if cpuErr == nil {
		for i, value := range utilization {
			a.gauges[fmt.Sprintf("CPUutilization%d", i+1)] = value
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
	if metrics := a.snapshot(); len(metrics) > 0 {
		a.sendMetrics(context.Background(), metrics)
	}
}

func (a *Agent) snapshot() []models.Metrics {
	a.mu.Lock()
	gauges := make(map[string]float64, len(a.gauges))
	maps.Copy(gauges, a.gauges)
	counters := make(map[string]int64, len(a.counters))
	maps.Copy(counters, a.counters)
	clear(a.counters)
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

	return metrics
}

func (a *Agent) sendMetrics(ctx context.Context, metrics []models.Metrics) {
	body, err := compressJSON(metrics)
	if err != nil {
		logger.Sugar.Errorf("failed to compress metrics: %s", err)
		return
	}

	url := fmt.Sprintf("%s/updates/", a.serverAddr)
	err = retry.DoWithContext(ctx, func() error {
		request := a.client.R().
			SetContext(ctx).
			SetHeader("Content-Type", "application/json").
			SetHeader("Content-Encoding", "gzip").
			SetHeader("Accept-Encoding", "gzip").
			SetBody(body)
		if a.key != "" {
			request.SetHeader(signature.Header, signature.Sign(body, a.key))
		}
		r, err := request.Post(url)
		if err != nil {
			return err
		}
		if r.StatusCode() != http.StatusOK {
			return fmt.Errorf("response status %s", r.Status())
		}
		return nil
	}, retry.IsConnectionError)
	if err != nil {
		logger.Sugar.Errorf("failed to send metrics: %s", err)
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
