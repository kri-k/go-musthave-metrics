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

func (s *MetricsService) GetAllMetrics() []models.Metric {
	gauges := s.repo.GetGauges()
	slices.SortFunc(gauges, func(a, b repository.GaugeMetric) int {
		return cmp.Compare(a.Name, b.Name)
	})

	counters := s.repo.GetCounters()
	slices.SortFunc(counters, func(a, b repository.CounterMetric) int {
		return cmp.Compare(a.Name, b.Name)
	})

	metrics := make([]models.Metric, 0, len(gauges)+len(counters))
	for _, g := range gauges {
		metrics = append(metrics, models.Metric{
			ID:    g.Name,
			MType: models.Gauge,
			Value: util.GaugeToString(g.Value),
		})
	}
	for _, c := range counters {
		metrics = append(metrics, models.Metric{
			ID:    c.Name,
			MType: models.Counter,
			Value: util.CounterToString(c.Value),
		})
	}
	return metrics
}

func (s *MetricsService) UpdateMetric(mType, name string, value string) error {
	switch mType {
	case models.Gauge:
		v, err := util.StringToGauge(value)
		if err != nil {
			return err
		}
		s.repo.UpdateGauge(name, v)
	case models.Counter:
		v, err := util.StringToCounter(value)
		if err != nil {
			return err
		}
		s.repo.UpdateCounter(name, v)
	default:
		return fmt.Errorf("unknown metric type: %s", mType)
	}
	return nil
}
