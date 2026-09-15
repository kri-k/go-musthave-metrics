package retry

import "time"

// Intervals — паузы перед тремя дополнительными попытками.
var Intervals = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	5 * time.Second,
}

var sleep = time.Sleep

// Predicate определяет, стоит ли повторять операцию после ошибки.
type Predicate func(error) bool

// Do выполняет fn и при retriable-ошибке повторяет её ещё три раза
// с интервалами 1s, 3s и 5s.
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
