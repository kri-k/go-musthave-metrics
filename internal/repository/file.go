package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	models "github.com/kri-k/go-musthave-metrics/internal/model"
)

func (s *MemStorage) SaveToFile(path string) error {
	if path == "" {
		return nil
	}

	s.mu.RLock()
	metrics := make([]models.Metrics, 0, len(s.gauges)+len(s.counters))
	for name, value := range s.gauges {
		v := value
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Gauge,
			Value: &v,
		})
	}
	for name, value := range s.counters {
		d := value
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Counter,
			Delta: &d,
		})
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}

	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create storage directory: %w", err)
		}
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write metrics: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func (s *MemStorage) LoadFromFile(path string) error {
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read metrics file: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	var metrics []models.Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return fmt.Errorf("unmarshal metrics: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range metrics {
		switch m.MType {
		case models.Gauge:
			if m.Value != nil {
				s.gauges[m.ID] = *m.Value
			}
		case models.Counter:
			if m.Delta != nil {
				s.counters[m.ID] = *m.Delta
			}
		}
	}
	return nil
}
