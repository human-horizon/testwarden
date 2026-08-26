package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_FirstAttemptSucceeds(t *testing.T) {
	count := 0
	err := Do(context.Background(), Default(), func() error {
		count++
		return nil
	})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 call, got %d", count)
	}
}

func TestDo_RetriesOnError(t *testing.T) {
	count := 0
	err := Do(context.Background(), Strategy{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	}, func() error {
		count++
		if count < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Errorf("expected nil after retries, got %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 calls, got %d", count)
	}
}

func TestDo_GivesUp(t *testing.T) {
	count := 0
	err := Do(context.Background(), Strategy{
		MaxAttempts: 2,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	}, func() error {
		count++
		return errors.New("always fails")
	})
	if err == nil {
		t.Error("expected error after exhausting attempts")
	}
	if count != 2 {
		t.Errorf("expected 2 calls, got %d", count)
	}
}

func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Do(ctx, Default(), func() error {
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestBackoff_CappedAtMax(t *testing.T) {
	for _, attempt := range []int{1, 2, 3, 10, 100} {
		d := backoff(time.Second, 5*time.Second, attempt)
		if d > 6*time.Second {
			t.Errorf("attempt %d: backoff %v exceeds max+jitter", attempt, d)
		}
	}
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("connection refused"), true},
		{errors.New("timeout waiting"), true},
		{errors.New("502 bad gateway"), true},
		{errors.New("permanent error"), false},
		{context.DeadlineExceeded, true},
	}
	for _, tt := range tests {
		got := IsTransient(tt.err)
		if got != tt.want {
			t.Errorf("IsTransient(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}
