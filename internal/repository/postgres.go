package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kri-k/go-musthave-metrics/internal/logger"
	models "github.com/kri-k/go-musthave-metrics/internal/model"
)

const queryTimeout = 3 * time.Second

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(db *sql.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

func (s *PostgresStorage) UpdateGauge(name string, value float64) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO metrics (id, mtype, value)
		VALUES ($1, $2, $3)
		ON CONFLICT (id, mtype) DO UPDATE SET value = EXCLUDED.value
	`, name, models.Gauge, value)
	if err != nil {
		logger.Sugar.Errorw("failed to update gauge", "id", name, "error", err)
	}
	return value
}

func (s *PostgresStorage) UpdateCounter(name string, value int64) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	var result int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO metrics (id, mtype, delta)
		VALUES ($1, $2, $3)
		ON CONFLICT (id, mtype) DO UPDATE SET delta = metrics.delta + EXCLUDED.delta
		RETURNING delta
	`, name, models.Counter, value).Scan(&result)
	if err != nil {
		logger.Sugar.Errorw("failed to update counter", "id", name, "error", err)
		return 0
	}
	return result
}

func (s *PostgresStorage) UpdateMetrics(metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		logger.Sugar.Errorw("failed to begin metrics transaction", "error", err)
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	gaugeStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO metrics (id, mtype, value)
		VALUES ($1, $2, $3)
		ON CONFLICT (id, mtype) DO UPDATE SET value = EXCLUDED.value
	`)
	if err != nil {
		logger.Sugar.Errorw("failed to prepare gauge statement", "error", err)
		return err
	}
	defer gaugeStmt.Close()

	counterStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO metrics (id, mtype, delta)
		VALUES ($1, $2, $3)
		ON CONFLICT (id, mtype) DO UPDATE SET delta = metrics.delta + EXCLUDED.delta
	`)
	if err != nil {
		logger.Sugar.Errorw("failed to prepare counter statement", "error", err)
		return err
	}
	defer counterStmt.Close()

	for _, m := range metrics {
		switch m.MType {
		case models.Gauge:
			if _, err := gaugeStmt.ExecContext(ctx, m.ID, models.Gauge, *m.Value); err != nil {
				logger.Sugar.Errorw("failed to update gauge", "id", m.ID, "error", err)
				return err
			}
		case models.Counter:
			if _, err := counterStmt.ExecContext(ctx, m.ID, models.Counter, *m.Delta); err != nil {
				logger.Sugar.Errorw("failed to update counter", "id", m.ID, "error", err)
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		logger.Sugar.Errorw("failed to commit metrics transaction", "error", err)
		return err
	}
	return nil
}

func (s *PostgresStorage) GetGauge(name string) (float64, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	var value float64
	err := s.db.QueryRowContext(ctx, `
		SELECT value FROM metrics WHERE id = $1 AND mtype = $2
	`, name, models.Gauge).Scan(&value)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logger.Sugar.Errorw("failed to get gauge", "id", name, "error", err)
		}
		return 0, false
	}
	return value, true
}

func (s *PostgresStorage) GetCounter(name string) (int64, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	var value int64
	err := s.db.QueryRowContext(ctx, `
		SELECT delta FROM metrics WHERE id = $1 AND mtype = $2
	`, name, models.Counter).Scan(&value)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logger.Sugar.Errorw("failed to get counter", "id", name, "error", err)
		}
		return 0, false
	}
	return value, true
}

func (s *PostgresStorage) GetGauges() []GaugeMetric {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, value FROM metrics WHERE mtype = $1
	`, models.Gauge)
	if err != nil {
		logger.Sugar.Errorw("failed to list gauges", "error", err)
		return nil
	}
	defer rows.Close()

	result := make([]GaugeMetric, 0)
	for rows.Next() {
		var m GaugeMetric
		if err := rows.Scan(&m.Name, &m.Value); err != nil {
			logger.Sugar.Errorw("failed to scan gauge", "error", err)
			return nil
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		logger.Sugar.Errorw("failed to iterate gauges", "error", err)
		return nil
	}
	return result
}

func (s *PostgresStorage) GetCounters() []CounterMetric {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, delta FROM metrics WHERE mtype = $1
	`, models.Counter)
	if err != nil {
		logger.Sugar.Errorw("failed to list counters", "error", err)
		return nil
	}
	defer rows.Close()

	result := make([]CounterMetric, 0)
	for rows.Next() {
		var m CounterMetric
		if err := rows.Scan(&m.Name, &m.Value); err != nil {
			logger.Sugar.Errorw("failed to scan counter", "error", err)
			return nil
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		logger.Sugar.Errorw("failed to iterate counters", "error", err)
		return nil
	}
	return result
}

var _ Repository = (*PostgresStorage)(nil)
