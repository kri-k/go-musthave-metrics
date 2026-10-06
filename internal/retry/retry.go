package retry

import (
	"context"
	"time"
)

var Intervals = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	5 * time.Second,
}

var sleep = time.Sleep

type Predicate func(error) bool

func Do(fn func() error, isRetriable Predicate) error {
	err := fn()
	if err == nil {
		return nil
	}

	for _, interval := range Intervals {
		if !isRetriable(err) {
			return err
		}
		sleep(interval)
		err = fn()
		if err == nil {
			return nil
		}
	}
	return err
}

// DoWithContext interrupts retries and backoff when the context is canceled.
func DoWithContext(ctx context.Context, fn func() error, isRetriable Predicate) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := fn()
	for _, interval := range Intervals {
		if err == nil || !isRetriable(err) {
			return err
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = fn()
	}
	return err
}
