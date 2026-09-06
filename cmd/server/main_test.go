package main

import (
	"testing"

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
