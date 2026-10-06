package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDoWithContextCancelsBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	started := time.Now()
	err := DoWithContext(ctx, func() error {
		calls++
		cancel()
		return errors.New("connection failed")
	}, func(error) bool { return true })
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
	require.Less(t, time.Since(started), time.Second)
}
