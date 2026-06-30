package agent

import (
	"context"
	"fmt"
	"log"
	"maps"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/kri-k/go-musthave-metrics/internal/util"
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
	log.Printf(
		"starting agent with settings:\nPollInterval: %ds\nReportInterval: %ds\nServerAddr: %s",
		a.pollInterval/time.Second, a.reportInterval/time.Second, a.serverAddr)

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

	log.Println("polling metrics...")
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

	log.Println("reporting metrics...")
	for name, value := range gauges {
		a.sendMetric("gauge", name, util.GaugeToString(value))
	}

	for name, value := range counters {
		a.sendMetric("counter", name, util.CounterToString(value))
	}
}

func (a *Agent) sendMetric(mType, name, value string) {
	url := fmt.Sprintf("%s/update/%s/%s/%s", a.serverAddr, mType, name, value)
	_, err := a.client.R().Post(url)
	if err != nil {
		log.Printf("failed to send metric: %s", err)
	}
}

// GetGauge returns a gauge metric value for testing.
func (a *Agent) GetGauge(name string) (float64, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	v, ok := a.gauges[name]
	return v, ok
}

// GetCounter returns a counter metric value for testing.
func (a *Agent) GetCounter(name string) (int64, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	v, ok := a.counters[name]
	return v, ok
}

// SetServerAddr sets the server address for testing.
func (a *Agent) SetServerAddr(addr string) {
	a.serverAddr = addr
}
