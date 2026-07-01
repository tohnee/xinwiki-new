package chat

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestCircuitBreaker_ClosedPassesThrough: in the closed state a call is
// executed and its result/error returned verbatim.
func TestCircuitBreaker_ClosedPassesThrough(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		OpenDuration:     time.Second,
	})
	called := false
	err := cb.Call(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("closed circuit must pass through, got %v", err)
	}
	if !called {
		t.Fatalf("call must be executed")
	}
}

// TestCircuitBreaker_FailuresTripOpen: reaching the failure threshold trips
// the breaker to open; subsequent calls are short-circuited with
// ErrCircuitOpen WITHOUT invoking the wrapped call.
func TestCircuitBreaker_FailuresTripOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		OpenDuration:     time.Second,
	})
	boom := errors.New("provider down")
	for i := 0; i < 3; i++ {
		if err := cb.Call(context.Background(), func(ctx context.Context) error { return boom }); err != nil {
			// failures propagate while still closed (until the threshold trip).
			if !errors.Is(err, boom) {
				t.Fatalf("unexpected err: %v", err)
			}
		}
	}
	// 3rd failure trips the breaker. Now a call must short-circuit.
	called := false
	err := cb.Call(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("tripped breaker must return ErrCircuitOpen, got %v", err)
	}
	if called {
		t.Fatalf("wrapped call must NOT execute when circuit is open")
	}
}

// TestCircuitBreaker_SuccessResetsCount: a success between failures resets the
// consecutive-failure counter so the breaker does not trip on scattered
// failures.
func TestCircuitBreaker_SuccessResetsCount(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		OpenDuration:     time.Second,
	})
	boom := errors.New("transient")
	// 2 failures, then a success, then 2 more failures -> still closed.
	_ = cb.Call(context.Background(), func(ctx context.Context) error { return boom })
	_ = cb.Call(context.Background(), func(ctx context.Context) error { return boom })
	_ = cb.Call(context.Background(), func(ctx context.Context) error { return nil })
	_ = cb.Call(context.Background(), func(ctx context.Context) error { return boom })
	_ = cb.Call(context.Background(), func(ctx context.Context) error { return boom })

	// Only 2 consecutive failures since the reset -> circuit must still be closed.
	called := false
	err := cb.Call(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})
	if err != nil || !called {
		t.Fatalf("circuit should remain closed after reset, err=%v called=%v", err, called)
	}
}

// TestCircuitBreaker_HalfOpenAllowsProbe: after OpenDuration elapses the
// breaker enters half-open and allows ONE probe call. A probe success closes
// the circuit; a probe failure re-opens it.
func TestCircuitBreaker_HalfOpenAllowsProbe(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDuration:     30 * time.Millisecond,
	})
	boom := errors.New("provider down")
	_ = cb.Call(context.Background(), func(ctx context.Context) error { return boom })
	_ = cb.Call(context.Background(), func(ctx context.Context) error { return boom })

	// Wait for the open window to expire.
	time.Sleep(60 * time.Millisecond)

	// Half-open: one probe call is allowed.
	called := false
	err := cb.Call(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})
	if err != nil || !called {
		t.Fatalf("half-open probe must be allowed, err=%v called=%v", err, called)
	}
	// Probe succeeded -> circuit closed again; next call passes through.
	called = false
	err = cb.Call(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})
	if err != nil || !called {
		t.Fatalf("circuit must be closed after a successful probe, err=%v called=%v", err, called)
	}
}

// TestCircuitBreaker_HalfOpenProbeFailureReopens: a failing probe re-opens the
// circuit immediately.
func TestCircuitBreaker_HalfOpenProbeFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDuration:     30 * time.Millisecond,
	})
	boom := errors.New("provider down")
	_ = cb.Call(context.Background(), func(ctx context.Context) error { return boom })
	_ = cb.Call(context.Background(), func(ctx context.Context) error { return boom })

	time.Sleep(60 * time.Millisecond)

	// Probe fails -> reopen.
	_ = cb.Call(context.Background(), func(ctx context.Context) error { return boom })

	// Now closed? No — reopened. Next call short-circuits.
	called := false
	err := cb.Call(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("failed probe must reopen circuit, got %v", err)
	}
	if called {
		t.Fatalf("wrapped call must NOT execute after failed probe")
	}
}

// TestCircuitBreaker_PropagatesContextCancel: the wrapped call receives the
// supplied context; a cancelled context surfaces the cancellation error.
func TestCircuitBreaker_PropagatesContextCancel(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenDuration:     time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := cb.Call(ctx, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// A context cancellation is NOT a provider failure -> must not count
	// toward the failure threshold.
	state := cb.State()
	if state == CircuitOpen {
		t.Fatalf("cancellation must not trip the breaker")
	}
}

// TestCircuitBreaker_ConcurrentSafe: the breaker is safe under concurrent
// calls (no data race, consistent state).
func TestCircuitBreaker_ConcurrentSafe(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1000,
		OpenDuration:     time.Second,
	})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cb.Call(context.Background(), func(ctx context.Context) error { return nil })
		}()
	}
	wg.Wait()
	if cb.State() != CircuitClosed {
		t.Fatalf("all-success concurrent run must keep circuit closed, got %v", cb.State())
	}
}

// TestCircuitBreaker_NilBreakerIsNoop: a nil breaker (the default when the
// feature is disabled) passes calls through unchanged so existing providers
// keep working without wiring a breaker.
func TestCircuitBreaker_NilBreakerIsNoop(t *testing.T) {
	var cb *CircuitBreaker // nil
	called := false
	err := cb.Call(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})
	if err != nil || !called {
		t.Fatalf("nil breaker must pass through, err=%v called=%v", err, called)
	}
}
