package repository

import "sync"

type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges[name] = value
}

func (s *MemStorage) UpdateCounter(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] = value
}

func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.gauges[name]
	return v, ok
}

func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.counters[name]
	return v, ok
}

func (s *MemStorage) GetGauges() []GaugeMetric {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]GaugeMetric, 0, len(s.gauges))
	for name, value := range s.gauges {
		result = append(result, GaugeMetric{Name: name, Value: value})
	}
	return result
}

func (s *MemStorage) GetCounters() []CounterMetric {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]CounterMetric, 0, len(s.counters))
	for name, value := range s.counters {
		result = append(result, CounterMetric{Name: name, Value: value})
	}
	return result
}

var _ Repository = (*MemStorage)(nil)
