package pgerrors

import (
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

type Classification int

const (
	NonRetriable Classification = iota
	Retriable
)

type Classifier struct{}

func New() *Classifier {
	return &Classifier{}
}

func (c *Classifier) Classify(err error) Classification {
	if err == nil {
		return NonRetriable
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return ClassifyPgError(pgErr)
	}

	if pgconn.SafeToRetry(err) || pgconn.Timeout(err) {
		return Retriable
	}

	return NonRetriable
}

func ClassifyPgError(pgErr *pgconn.PgError) Classification {
	// Коды ошибок PostgreSQL: https://www.postgresql.org/docs/current/errcodes-appendix.html
	if pgerrcode.IsConnectionException(pgErr.Code) {
		return Retriable
	}

	switch pgErr.Code {
	// Класс 40 — откат транзакции.
	case pgerrcode.TransactionRollback,
		pgerrcode.SerializationFailure,
		pgerrcode.DeadlockDetected:
		return Retriable

	// Класс 57 — ошибка оператора.
	case pgerrcode.CannotConnectNow:
		return Retriable
	}

	switch pgErr.Code {
	// Класс 22 — ошибки данных.
	case pgerrcode.DataException,
		pgerrcode.NullValueNotAllowedDataException:
		return NonRetriable

	// Класс 23 — нарушение ограничений целостности.
	case pgerrcode.IntegrityConstraintViolation,
		pgerrcode.RestrictViolation,
		pgerrcode.NotNullViolation,
		pgerrcode.ForeignKeyViolation,
		pgerrcode.UniqueViolation,
		pgerrcode.CheckViolation:
		return NonRetriable

	// Класс 42 — синтаксические ошибки.
	case pgerrcode.SyntaxErrorOrAccessRuleViolation,
		pgerrcode.SyntaxError,
		pgerrcode.UndefinedColumn,
		pgerrcode.UndefinedTable,
		pgerrcode.UndefinedFunction:
		return NonRetriable
	}

	return NonRetriable
}
