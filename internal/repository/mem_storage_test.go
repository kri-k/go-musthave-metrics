package repository

import (
	"sync"
	"testing"

	models "github.com/kri-k/go-musthave-metrics/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemStorage_UpdateMetrics(t *testing.T) {
	s := NewMemStorage()

	gauge := 10.5
	delta := int64(2)
	err := s.UpdateMetrics([]models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &gauge},
		{ID: "PollCount", MType: models.Counter, Delta: &delta},
		{ID: "PollCount", MType: models.Counter, Delta: &delta},
	})
	require.NoError(t, err)

	v, ok := s.GetGauge("Alloc")
	require.True(t, ok)
	assert.Equal(t, 10.5, v)

	c, ok := s.GetCounter("PollCount")
	require.True(t, ok)
	assert.Equal(t, int64(4), c)
}

func TestMemStorage_UpdateMetricsConcurrent(t *testing.T) {
	s := NewMemStorage()
	delta := int64(1)

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.UpdateMetrics([]models.Metrics{
				{ID: "PollCount", MType: models.Counter, Delta: &delta},
			})
		}()
	}
	wg.Wait()

	c, ok := s.GetCounter("PollCount")
	require.True(t, ok)
	assert.Equal(t, int64(50), c)
}
