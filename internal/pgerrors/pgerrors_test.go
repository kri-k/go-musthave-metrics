package pgerrors

import (
	"fmt"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestClassify(t *testing.T) {
	classifier := NewPostgresErrorClassifier()

	tests := []struct {
		name string
		err  error
		want PGErrorClassification
	}{
		{
			name: "nil",
			err:  nil,
			want: NonRetriable,
		},
		{
			name: "connection exception class 08",
			err:  &pgconn.PgError{Code: pgerrcode.ConnectionFailure},
			want: Retriable,
		},
		{
			name: "connection does not exist",
			err:  &pgconn.PgError{Code: pgerrcode.ConnectionDoesNotExist},
			want: Retriable,
		},
		{
			name: "sqlclient unable to establish connection",
			err:  &pgconn.PgError{Code: pgerrcode.SQLClientUnableToEstablishSQLConnection},
			want: Retriable,
		},
		{
			name: "cannot connect now",
			err:  &pgconn.PgError{Code: pgerrcode.CannotConnectNow},
			want: Retriable,
		},
		{
			name: "serialization failure",
			err:  &pgconn.PgError{Code: pgerrcode.SerializationFailure},
			want: Retriable,
		},
		{
			name: "unique violation",
			err:  &pgconn.PgError{Code: pgerrcode.UniqueViolation},
			want: NonRetriable,
		},
		{
			name: "syntax error",
			err:  &pgconn.PgError{Code: pgerrcode.SyntaxError},
			want: NonRetriable,
		},
		{
			name: "wrapped connection failure",
			err:  fmt.Errorf("query failed: %w", &pgconn.PgError{Code: pgerrcode.ConnectionException}),
			want: Retriable,
		},
		{
			name: "unknown error",
			err:  fmt.Errorf("something else"),
			want: NonRetriable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, classifier.Classify(tt.err))
		})
	}
}
