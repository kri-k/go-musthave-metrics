package agent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	models "github.com/kri-k/go-musthave-metrics/internal/model"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/stretchr/testify/require"
)

func TestRunLimitsRequestsAndKeepsPolling(t *testing.T) {
	for _, limit := range []int{1, 3} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			var active, maximum atomic.Int32
			entered := make(chan struct{}, 100)
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				n := active.Add(1)
				defer active.Add(-1)
				for old := maximum.Load(); n > old; old = maximum.Load() {
					if maximum.CompareAndSwap(old, n) {
						break
					}
				}
				entered <- struct{}{}
				<-release
			}))
			defer server.Close()
			defer close(release)
			a := NewAgentWithRateLimit(server.URL, 1, 1, limit, "")
			a.pollInterval = 5 * time.Millisecond
			a.reportInterval = 10 * time.Millisecond
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() { a.Run(ctx); close(done) }()
			for i := 0; i < limit; i++ {
				select {
				case <-entered:
				case <-time.After(3 * time.Second):
					t.Fatal("workers did not start requests")
				}
			}
			previous, _ := a.GetGaugeForTest("RandomValue")
			require.Eventually(t, func() bool {
				current, _ := a.GetGaugeForTest("RandomValue")
				return current != previous
			}, time.Second, 5*time.Millisecond)
			select {
			case <-entered:
				t.Fatal("request limit exceeded")
			case <-time.After(100 * time.Millisecond):
			}
			require.Equal(t, int32(limit), maximum.Load())
			cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("Run did not cancel in-flight requests")
			}
		})
	}
}

func TestPollSystem(t *testing.T) {
	a := NewAgent()
	a.pollSystem(context.Background())
	total, ok := a.GetGaugeForTest("TotalMemory")
	require.True(t, ok)
	require.Positive(t, total)
	free, ok := a.GetGaugeForTest("FreeMemory")
	require.True(t, ok)
	require.GreaterOrEqual(t, free, float64(0))
	require.LessOrEqual(t, free, total)
	counts, err := cpu.Counts(true)
	require.NoError(t, err)
	for i := 1; i <= counts; i++ {
		value, ok := a.GetGaugeForTest(fmt.Sprintf("CPUutilization%d", i))
		require.True(t, ok)
		require.GreaterOrEqual(t, value, float64(0))
		require.LessOrEqual(t, value, float64(100))
	}
}

func TestSnapshotConsumesCounterOnce(t *testing.T) {
	a := NewAgent()
	a.Poll()
	first := a.snapshot()
	a.Poll()
	second := a.snapshot()
	for _, metrics := range [][]models.Metrics{first, second} {
		found := false
		for _, metric := range metrics {
			if metric.ID == "PollCount" {
				require.Equal(t, int64(1), *metric.Delta)
				found = true
			}
		}
		require.True(t, found)
	}

	for _, metric := range a.snapshot() {
		require.NotEqual(t, "PollCount", metric.ID)
	}
}
