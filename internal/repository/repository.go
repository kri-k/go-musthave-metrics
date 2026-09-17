package repository

import models "github.com/kri-k/go-musthave-metrics/internal/model"

type GaugeMetric struct {
	Name  string
	Value float64
}

type CounterMetric struct {
	Name  string
	Value int64
}

type Repository interface {
	UpdateGauge(name string, value float64) float64
	UpdateCounter(name string, value int64) int64
	UpdateMetrics(metrics []models.Metrics) error
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetGauges() []GaugeMetric
	GetCounters() []CounterMetric
}
