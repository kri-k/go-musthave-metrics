package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/kri-k/go-musthave-metrics/internal/repository"
	"github.com/stretchr/testify/require"
)

func TestNewRepositoryFallback(t *testing.T) {
	t.Run("memory when file path is empty", func(t *testing.T) {
		repo, mem := newRepository(nil, "", 300, false)
		require.NotNil(t, repo)
		require.NotNil(t, mem)
		_, ok := repo.(*repository.MemStorage)
		require.True(t, ok)
	})

	t.Run("sync file when interval is zero", func(t *testing.T) {
		repo, mem := newRepository(nil, "./metrics-db.json", 0, false)
		require.NotNil(t, mem)
		_, ok := repo.(*repository.SyncFileStorage)
		require.True(t, ok)
	})

	t.Run("async file when interval is positive", func(t *testing.T) {
		repo, mem := newRepository(nil, "./metrics-db.json", 10, false)
		require.Same(t, mem, repo)
	})
}

func TestMainRejectsUnavailableDatabase(t *testing.T) {
	if os.Getenv("METRICS_TEST_STARTUP") == "1" {
		// Testing flags have already been parsed; main must only see server flags.
		os.Args = os.Args[:1]
		main()
		return
	}

	// A nonexistent Unix socket gives a deterministic connection failure without PostgreSQL.
	t.Setenv("DATABASE_DSN", "host="+t.TempDir()+" dbname=metrics connect_timeout=1")
	t.Setenv("ADDRESS", "127.0.0.1:0")
	t.Setenv("STORE_INTERVAL", "0")
	t.Setenv("RESTORE", "false")
	t.Setenv("FILE_STORAGE_PATH", "")
	t.Setenv("METRICS_TEST_STARTUP", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMainRejectsUnavailableDatabase$")
	output, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err(), "server must exit instead of serving with fallback storage")
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	require.Equal(t, 1, exitErr.ExitCode())
	require.Contains(t, string(output), "failed to initialize database")
	require.Contains(t, string(output), "ping database")
	require.NotContains(t, string(output), "starting server")
}
