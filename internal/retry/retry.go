// Package retry provides exponential-backoff retry for transient failures.
package retry

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// Strategy controls retry behaviour.
type Strategy struct {
	MaxAttempts int           // total attempts including the first try
	BaseDelay   time.Duration // initial delay before the second attempt
	MaxDelay    time.Duration // cap on individual backoff
}

// Default returns a sensible default: 3 attempts, 1s base, 30s cap.
func Default() Strategy {
	return Strategy{
		MaxAttempts: 3,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
	}
}

// Do runs fn up to strategy.MaxAttempts times, sleeping between attempts
// with exponential backoff + jitter. Stops early on context cancellation
// or when fn returns nil.
func Do(ctx context.Context, s Strategy, fn func() error) error {
	if s.MaxAttempts < 1 {
		s.MaxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= s.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == s.MaxAttempts {
			break
		}

		delay := backoff(s.BaseDelay, s.MaxDelay, attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

// backoff returns BaseDelay * 2^(attempt-1), capped at MaxDelay, plus jitter.
func backoff(base, max time.Duration, attempt int) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	if max <= 0 {
		max = 30 * time.Second
	}
	exp := base << (attempt - 1)
	if exp <= 0 || exp > max {
		exp = max
	}
	jitter := time.Duration(rand.Int63n(int64(base)))
	return exp + jitter
}

// IsTransient reports whether err looks like a transient failure worth retrying.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, marker := range []string{
		"timeout", "connection refused", "EOF",
		"reset by peer", "temporarily unavailable",
		"429", "502", "503", "504",
	} {
		if contains(msg, marker) {
			return true
		}
	}
	return errors.Is(err, context.DeadlineExceeded)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
