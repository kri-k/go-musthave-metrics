package service

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	models "github.com/kri-k/go-musthave-metrics/internal/model"
	"github.com/kri-k/go-musthave-metrics/internal/repository"
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
			return GaugeToString(v), nil
		}
		return "", fmt.Errorf("gauge %s not found", name)
	case models.Counter:
		if v, ok := s.repo.GetCounter(name); ok {
			return CounterToString(v), nil
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
			Value: GaugeToString(g.Value),
		})
	}
	for _, c := range counters {
		metrics = append(metrics, models.Metric{
			ID:    c.Name,
			MType: models.Counter,
			Value: CounterToString(c.Value),
		})
	}
	return metrics
}

func (s *MetricsService) UpdateMetric(mType, name string, value string) error {
	switch mType {
	case models.Gauge:
		v, err := StringToGauge(value)
		if err != nil {
			return err
		}
		s.repo.UpdateGauge(name, v)
	case models.Counter:
		v, err := StringToCounter(value)
		if err != nil {
			return err
		}
		s.repo.UpdateCounter(name, v)
	default:
		return fmt.Errorf("unknown metric type: %s", mType)
	}
	return nil
}

func GaugeToString(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func CounterToString(value int64) string {
	return strconv.FormatInt(value, 10)
}

func StringToGauge(value string) (float64, error) {
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid gauge value: %w", err)
	}
	return v, nil
}

func StringToCounter(value string) (int64, error) {
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid counter value: %w", err)
	}
	return v, nil
}
