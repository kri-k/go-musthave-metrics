package retry

import "time"

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
