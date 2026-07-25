package service

import (
	"cmp"
	"fmt"
	"slices"

	models "github.com/kri-k/go-musthave-metrics/internal/model"
	"github.com/kri-k/go-musthave-metrics/internal/repository"
	"github.com/kri-k/go-musthave-metrics/internal/util"
)

type MetricsService struct {
	repo repository.Repository
}

func NewMetricsService(repo repository.Repository) *MetricsService {
	return &MetricsService{repo: repo}
}

func (s *MetricsService) GetMetric(mType, name string) (string, error) {
	switch mType {
	case models.Gauge:
		if v, ok := s.repo.GetGauge(name); ok {
			return util.GaugeToString(v), nil
		}
		return "", fmt.Errorf("gauge %s not found", name)
	case models.Counter:
		if v, ok := s.repo.GetCounter(name); ok {
			return util.CounterToString(v), nil
		}
		return "", fmt.Errorf("counter %s not found", name)
	default:
		return "", fmt.Errorf("unknown metric type: %s", mType)
	}
}

func (s *MetricsService) GetMetricJSON(mType, name string) (models.Metrics, error) {
	switch mType {
	case models.Gauge:
		if v, ok := s.repo.GetGauge(name); ok {
			return models.Metrics{ID: name, MType: models.Gauge, Value: &v}, nil
		}
		return models.Metrics{}, fmt.Errorf("gauge %s not found", name)
	case models.Counter:
		if v, ok := s.repo.GetCounter(name); ok {
			return models.Metrics{ID: name, MType: models.Counter, Delta: &v}, nil
		}
		return models.Metrics{}, fmt.Errorf("counter %s not found", name)
	default:
		return models.Metrics{}, fmt.Errorf("unknown metric type: %s", mType)
	}
}

func (s *MetricsService) GetAllMetrics() []models.Metrics {
	gauges := s.repo.GetGauges()
	slices.SortFunc(gauges, func(a, b repository.GaugeMetric) int {
		return cmp.Compare(a.Name, b.Name)
	})

	counters := s.repo.GetCounters()
	slices.SortFunc(counters, func(a, b repository.CounterMetric) int {
		return cmp.Compare(a.Name, b.Name)
	})

	metrics := make([]models.Metrics, 0, len(gauges)+len(counters))
	for _, g := range gauges {
		v := g.Value
		metrics = append(metrics, models.Metrics{
			ID:    g.Name,
			MType: models.Gauge,
			Value: &v,
		})
	}
	for _, c := range counters {
		d := c.Value
		metrics = append(metrics, models.Metrics{
			ID:    c.Name,
			MType: models.Counter,
			Delta: &d,
		})
	}
	return metrics
}

func (s *MetricsService) UpdateMetric(mType, name string, value string) (string, error) {
	switch mType {
	case models.Gauge:
		v, err := util.StringToGauge(value)
		if err != nil {
			return "", err
		}
		updatedValue := s.repo.UpdateGauge(name, v)
		return util.GaugeToString(updatedValue), nil
	case models.Counter:
		v, err := util.StringToCounter(value)
		if err != nil {
			return "", err
		}
		updatedValue := s.repo.UpdateCounter(name, v)
		return util.CounterToString(updatedValue), nil
	default:
		return "", fmt.Errorf("unknown metric type: %s", mType)
	}
}

func (s *MetricsService) UpdateMetricJSON(m models.Metrics) (models.Metrics, error) {
	if m.ID == "" {
		return models.Metrics{}, fmt.Errorf("metric id is required")
	}

	switch m.MType {
	case models.Gauge:
		if m.Value == nil {
			return models.Metrics{}, fmt.Errorf("value is required for gauge")
		}
		v := s.repo.UpdateGauge(m.ID, *m.Value)
		return models.Metrics{ID: m.ID, MType: models.Gauge, Value: &v}, nil
	case models.Counter:
		if m.Delta == nil {
			return models.Metrics{}, fmt.Errorf("delta is required for counter")
		}
		d := s.repo.UpdateCounter(m.ID, *m.Delta)
		return models.Metrics{ID: m.ID, MType: models.Counter, Delta: &d}, nil
	default:
		return models.Metrics{}, fmt.Errorf("unknown metric type: %s", m.MType)
	}
}
