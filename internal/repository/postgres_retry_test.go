package repository

import (
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestIsRetriablePostgresError(t *testing.T) {
	assert.True(t, isRetriablePostgresError(&pgconn.PgError{Code: pgerrcode.ConnectionFailure}))
	assert.True(t, isRetriablePostgresError(fmt.Errorf("wrap: %w", &pgconn.PgError{Code: pgerrcode.ConnectionException})))
	assert.True(t, isRetriablePostgresError(&net.OpError{Op: "read", Err: syscall.ECONNRESET}))
	assert.False(t, isRetriablePostgresError(&pgconn.PgError{Code: pgerrcode.UniqueViolation}))
	assert.False(t, isRetriablePostgresError(fmt.Errorf("syntax error")))
}
