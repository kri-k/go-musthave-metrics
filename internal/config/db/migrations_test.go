package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kri-k/go-musthave-metrics/internal/handler"
	"github.com/stretchr/testify/require"
)

func TestRunMigrationsKeepsDatabaseOpen(t *testing.T) {
	database := sql.OpenDB(migrationConnector{})
	t.Cleanup(func() { _ = database.Close() })
	database.SetMaxOpenConns(1)

	// Repeat to cover startup when migrations are already applied.
	for range 2 {
		require.NoError(t, runMigrations(database))
		require.Zero(t, database.Stats().InUse, "migration connection must be returned to the pool")

		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/ping", nil)
		handler.NewPingHandler(database).Ping(response, request)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	}
}

// Simulate an available database with migration 1 already applied. The real
// database/sql pool and migrate driver manage connection ownership in this test.
type migrationConnector struct{}

func (migrationConnector) Connect(context.Context) (driver.Conn, error) {
	return migrationConn{}, nil
}
func (migrationConnector) Driver() driver.Driver { return migrationDriver{} }

type migrationDriver struct{}

func (migrationDriver) Open(string) (driver.Conn, error) { return migrationConn{}, nil }

type migrationConn struct{}

func (migrationConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("unexpected Prepare")
}
func (migrationConn) Begin() (driver.Tx, error)  { return nil, fmt.Errorf("unexpected Begin") }
func (migrationConn) Close() error               { return nil }
func (migrationConn) Ping(context.Context) error { return nil }

func (migrationConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if strings.HasPrefix(query, "SELECT pg_advisory_") {
		return driver.RowsAffected(1), nil
	}
	return nil, fmt.Errorf("unexpected exec: %s", query)
}

func (migrationConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case query == "SELECT CURRENT_DATABASE()":
		return &migrationRows{columns: []string{"database"}, values: []driver.Value{"metrics"}}, nil
	case query == "SELECT CURRENT_SCHEMA()":
		return &migrationRows{columns: []string{"schema"}, values: []driver.Value{"public"}}, nil
	case strings.HasPrefix(query, "SELECT COUNT(1) FROM information_schema.tables"):
		return &migrationRows{columns: []string{"count"}, values: []driver.Value{int64(1)}}, nil
	case strings.HasPrefix(query, "SELECT version, dirty FROM"):
		return &migrationRows{columns: []string{"version", "dirty"}, values: []driver.Value{int64(1), false}}, nil
	default:
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
}

type migrationRows struct {
	columns []string
	values  []driver.Value
}

func (r *migrationRows) Columns() []string { return r.columns }
func (r *migrationRows) Close() error      { return nil }
func (r *migrationRows) Next(dest []driver.Value) error {
	if r.values == nil {
		return io.EOF
	}
	copy(dest, r.values)
	r.values = nil
	return nil
}
