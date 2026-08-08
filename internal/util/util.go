package util

import (
	"fmt"
	"os"
	"strconv"
)

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

func GetEnvOrDefaultString(envVar, defaultValue string) string {
	v, _ := GetEnvOrDefault(envVar, defaultValue, func(s string) (string, error) { return s, nil })
	return v
}

func GetEnvOrDefault[T any](envVar string, defaultValue T, converter func(string) (T, error)) (T, error) {
	if envValue := os.Getenv(envVar); envValue != "" {
		v, err := converter(envValue)
		if err != nil {
			return defaultValue, err
		}
		return v, nil
	}
	return defaultValue, nil
}
