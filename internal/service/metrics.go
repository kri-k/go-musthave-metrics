package service

import (
	"fmt"
	"strconv"

	"github.com/kri-k/go-musthave-metrics/internal/model"
	"github.com/kri-k/go-musthave-metrics/internal/repository"
)

type MetricsService struct {
	repo repository.Repository
}

func NewMetricsService(repo repository.Repository) *MetricsService {
	return &MetricsService{repo: repo}
}

func (s *MetricsService) UpdateMetric(mType, name string, value string) error {
	switch mType {
	case models.Gauge:
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid gauge value: %w", err)
		}
		s.repo.UpdateGauge(name, v)
	case models.Counter:
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid counter value: %w", err)
		}
		s.repo.UpdateCounter(name, v)
	default:
		return fmt.Errorf("unknown metric type: %s", mType)
	}
	return nil
}
