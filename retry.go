package retry

import (
	"math"
	"time"
)

// Waiter is an interface that determines the amount of time to wait
// in between retries given the number of attempts and error that occurred
type Waiter interface {
	Wait(attempt uint, err error) time.Duration
}

// WaiterFunc is a function that can act as a Waiter
type WaiterFunc func(attempt uint, err error) time.Duration

// Wait returns the amount of time to wait given the attempt
func (w WaiterFunc) Wait(attempt uint, err error) time.Duration {
	return w(attempt, err)
}

// Duration is a WaiterFunc that waits for a static duration
func Duration(d time.Duration) WaiterFunc {
	return func(uint, error) time.Duration {
		return d
	}
}

// IncrementalBackoff is a WaiterFunc that waits for a duration d multiplied by
// the number of attempts that have occurred for the Retriable
// ex: 1, 2, 3, 4, 5 * time.Seconds
func IncrementalBackoff(d time.Duration) WaiterFunc {
	return func(attempt uint, _ error) time.Duration {
		return time.Duration(attempt) * d
	}
}

// ExponentialBackoff is a WaiterFunc that waits for a duration d multiplied by
// the number of attempts^2 that have occurred for the Retriable
// ex: 1, 4, 9, 16, 25 * time.Seconds
func ExponentialBackoff(d time.Duration) WaiterFunc {
	return func(attempt uint, _ error) time.Duration {
		wait := math.Pow(float64(attempt), 2.0)
		return time.Duration(wait) * d
	}
}

type sleeper interface {
	Sleep(time.Duration)
}

type defaultSleeper struct{}

func (s defaultSleeper) Sleep(d time.Duration) {
	time.Sleep(d)
}

// to allow for testing the duration sent to Sleep
var sleep sleeper = defaultSleeper{}

// Retriable represents an action that is retriable given an error
type Retriable struct {
	retries, maxAttempts uint
	maxWait              time.Duration
	waiter               Waiter
}

// Option is a functional option for a Retriable
type Option func(r *Retriable)

// Default creates a new Retriable that retries 3 times, sleeping for a second between each retry
func Default() *Retriable {
	return &Retriable{
		maxAttempts: 3,
		waiter:      Duration(time.Second),
	}
}

// New creates a new Retriable with optional functional options
// Note: the default Retriable retries 3 times, sleeping for a second between each retry
func New(opts ...Option) *Retriable {
	r := Default()
	for _, o := range opts {
		o(r)
	}
	return r
}

// WithMaxWait sets the maximum amount of duration to wait between retries,
// regardless of what is returned from the Waiter
func WithMaxWait(maxDuration time.Duration) Option {
	return func(r *Retriable) {
		r.maxWait = maxDuration
	}
}

// WithMaxAttempts sets the maximum number of attempts for a Retriable
func WithMaxAttempts(maxAttempts uint) Option {
	return func(r *Retriable) {
		r.maxAttempts = maxAttempts
	}
}

// WithWaiter sets the waiter used to determine the amount of time
// to wait between retries for a Retriable
func WithWaiter(waiter Waiter) Option {
	return func(r *Retriable) {
		r.waiter = waiter
	}
}

// WithDuration sets the waiter to use a Duration WaiterFunc with the specified duration
func WithDuration(d time.Duration) Option {
	return func(r *Retriable) {
		r.waiter = Duration(d)
	}
}

// WithIncrementalBackoff sets the waiter to use an IncrementalBackoff WaiterFunc with the specified duration
func WithIncrementalBackoff(d time.Duration) Option {
	return func(r *Retriable) {
		r.waiter = IncrementalBackoff(d)
	}
}

// WithExponentialBackoff sets the waiter to use an ExponentialBackoff WaiterFunc with the specified duration
func WithExponentialBackoff(d time.Duration) Option {
	return func(r *Retriable) {
		r.waiter = ExponentialBackoff(d)
	}
}

// Do calls the function f provided, retrying on any errors according
// to the properties defined on the Retriable r
func (r *Retriable) Do(f func() error) error {
	var (
		err      error
		retrying = true
	)

RETRY:
	for retrying {
		err = f()

		// if success
		if err == nil {
			return nil
		}

		// don't retry permanent errors
		if _, ok := err.(PermanentError); ok {
			return err
		}

		// if retriable
		retrying = r.retries < r.maxAttempts
		if retrying {
			duration := r.waiter.Wait(r.retries, err)
			if r.maxWait > 0 {
				duration = min(duration, r.maxWait)
			}

			sleep.Sleep(duration)
			r.retries++
			continue RETRY
		}

		// if not retriable
		return err
	}

	return err
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// PermanentError signals that the operation should not be retried
type PermanentError struct {
	error
}

// Permanent wraps the given err in a PermanentError
func Permanent(err error) PermanentError {
	return PermanentError{err}
}
