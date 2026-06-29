package models

const (
	Counter = "counter"
	Gauge   = "gauge"
)

type Metric struct {
	ID    string `json:"id"`
	MType string `json:"type"`
	Value string `json:"value"`
}
