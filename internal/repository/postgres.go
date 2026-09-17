package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kri-k/go-musthave-metrics/internal/logger"
	models "github.com/kri-k/go-musthave-metrics/internal/model"
	"github.com/kri-k/go-musthave-metrics/internal/pgerrors"
	"github.com/kri-k/go-musthave-metrics/internal/retry"
)

const queryTimeout = 3 * time.Second

var pgClassifier = pgerrors.NewPostgresErrorClassifier()

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(db *sql.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

func (s *PostgresStorage) withRetry(fn func(ctx context.Context) error) error {
	return retry.Do(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
		defer cancel()
		return fn(ctx)
	}, isRetriablePostgresError)
}

func isRetriablePostgresError(err error) bool {
	return pgClassifier.Classify(err) == pgerrors.Retriable || retry.IsConnectionError(err)
}

func (s *PostgresStorage) UpdateGauge(name string, value float64) float64 {
	err := s.withRetry(func(ctx context.Context) error {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO metrics (id, mtype, value)
			VALUES ($1, $2, $3)
			ON CONFLICT (id, mtype) DO UPDATE SET value = EXCLUDED.value
		`, name, models.Gauge, value)
		return err
	})
	if err != nil {
		logger.Sugar.Errorw("failed to update gauge", "id", name, "error", err)
	}
	return value
}

func (s *PostgresStorage) UpdateCounter(name string, value int64) int64 {
	var result int64
	err := s.withRetry(func(ctx context.Context) error {
		return s.db.QueryRowContext(ctx, `
			INSERT INTO metrics (id, mtype, delta)
			VALUES ($1, $2, $3)
			ON CONFLICT (id, mtype) DO UPDATE SET delta = metrics.delta + EXCLUDED.delta
			RETURNING delta
		`, name, models.Counter, value).Scan(&result)
	})
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

	err := s.withRetry(func(ctx context.Context) error {
		return s.updateMetricsTx(ctx, metrics)
	})
	if err != nil {
		logger.Sugar.Errorw("failed to update metrics", "error", err)
	}
	return err
}

func (s *PostgresStorage) updateMetricsTx(ctx context.Context, metrics []models.Metrics) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
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
		return err
	}
	defer gaugeStmt.Close()

	counterStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO metrics (id, mtype, delta)
		VALUES ($1, $2, $3)
		ON CONFLICT (id, mtype) DO UPDATE SET delta = metrics.delta + EXCLUDED.delta
	`)
	if err != nil {
		return err
	}
	defer counterStmt.Close()

	for _, m := range metrics {
		switch m.MType {
		case models.Gauge:
			if _, err := gaugeStmt.ExecContext(ctx, m.ID, models.Gauge, *m.Value); err != nil {
				return err
			}
		case models.Counter:
			if _, err := counterStmt.ExecContext(ctx, m.ID, models.Counter, *m.Delta); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *PostgresStorage) GetGauge(name string) (float64, bool) {
	var value float64
	err := s.withRetry(func(ctx context.Context) error {
		return s.db.QueryRowContext(ctx, `
			SELECT value FROM metrics WHERE id = $1 AND mtype = $2
		`, name, models.Gauge).Scan(&value)
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logger.Sugar.Errorw("failed to get gauge", "id", name, "error", err)
		}
		return 0, false
	}
	return value, true
}

func (s *PostgresStorage) GetCounter(name string) (int64, bool) {
	var value int64
	err := s.withRetry(func(ctx context.Context) error {
		return s.db.QueryRowContext(ctx, `
			SELECT delta FROM metrics WHERE id = $1 AND mtype = $2
		`, name, models.Counter).Scan(&value)
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logger.Sugar.Errorw("failed to get counter", "id", name, "error", err)
		}
		return 0, false
	}
	return value, true
}

func (s *PostgresStorage) GetGauges() []GaugeMetric {
	var result []GaugeMetric
	err := s.withRetry(func(ctx context.Context) error {
		rows, err := s.db.QueryContext(ctx, `
			SELECT id, value FROM metrics WHERE mtype = $1
		`, models.Gauge)
		if err != nil {
			return err
		}
		defer rows.Close()

		items := make([]GaugeMetric, 0)
		for rows.Next() {
			var m GaugeMetric
			if err := rows.Scan(&m.Name, &m.Value); err != nil {
				return err
			}
			items = append(items, m)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		result = items
		return nil
	})
	if err != nil {
		logger.Sugar.Errorw("failed to list gauges", "error", err)
		return nil
	}
	return result
}

func (s *PostgresStorage) GetCounters() []CounterMetric {
	var result []CounterMetric
	err := s.withRetry(func(ctx context.Context) error {
		rows, err := s.db.QueryContext(ctx, `
			SELECT id, delta FROM metrics WHERE mtype = $1
		`, models.Counter)
		if err != nil {
			return err
		}
		defer rows.Close()

		items := make([]CounterMetric, 0)
		for rows.Next() {
			var m CounterMetric
			if err := rows.Scan(&m.Name, &m.Value); err != nil {
				return err
			}
			items = append(items, m)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		result = items
		return nil
	})
	if err != nil {
		logger.Sugar.Errorw("failed to list counters", "error", err)
		return nil
	}
	return result
}

var _ Repository = (*PostgresStorage)(nil)
