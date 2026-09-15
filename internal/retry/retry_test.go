package retry

import (
	"errors"
	"io"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDo_RetriesRetriableError(t *testing.T) {
	var delays []time.Duration
	oldSleep := sleep
	sleep = func(d time.Duration) { delays = append(delays, d) }
	t.Cleanup(func() { sleep = oldSleep })

	attempts := 0
	retriable := errors.New("connection refused")
	err := Do(func() error {
		attempts++
		if attempts < 4 {
			return retriable
		}
		return nil
	}, func(err error) bool { return err != nil })

	require.NoError(t, err)
	assert.Equal(t, 4, attempts)
	assert.Equal(t, Intervals, delays)
}

func TestDo_StopsOnNonRetriableError(t *testing.T) {
	oldSleep := sleep
	sleep = func(time.Duration) { t.Fatal("should not sleep on non-retriable error") }
	t.Cleanup(func() { sleep = oldSleep })

	permanent := errors.New("bad request")
	attempts := 0
	err := Do(func() error {
		attempts++
		return permanent
	}, func(error) bool { return false })

	require.EqualError(t, err, permanent.Error())
	assert.Equal(t, 1, attempts)
}

func TestDo_ReturnsLastErrorAfterRetries(t *testing.T) {
	oldSleep := sleep
	sleep = func(time.Duration) {}
	t.Cleanup(func() { sleep = oldSleep })

	attempts := 0
	err := Do(func() error {
		attempts++
		return errors.New("still down")
	}, func(error) bool { return true })

	require.EqualError(t, err, "still down")
	assert.Equal(t, 4, attempts)
}

func TestIsConnectionError(t *testing.T) {
	assert.True(t, IsConnectionError(&net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}))
	assert.True(t, IsConnectionError(syscall.ECONNRESET))
	assert.True(t, IsConnectionError(io.EOF))
	assert.False(t, IsConnectionError(errors.New("response status 400 Bad Request")))
	assert.False(t, IsConnectionError(nil))
}
