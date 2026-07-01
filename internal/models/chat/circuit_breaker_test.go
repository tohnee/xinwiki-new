package chat

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_HealthyCallsSucceed(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		OpenDuration:     100 * time.Millisecond,
	})

	for i := 0; i < 10; i++ {
		err := cb.Call(context.Background(), func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Fatalf("call %d should succeed, got %v", i, err)
		}
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected closed, got %s", cb.State())
	}
}

func TestCircuitBreaker_TripsAfterConsecutiveFailures(t *testing.T) {
	resetBreakerRegistry()
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		OpenDuration:     1 * time.Second,
	})

	failErr := errors.New("provider down")
	for i := 0; i < 3; i++ {
		err := cb.Call(context.Background(), func(ctx context.Context) error {
			return failErr
		})
		if !errors.Is(err, failErr) {
			t.Fatalf("call %d should return failErr, got %v", i, err)
		}
	}

	err := cb.Call(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen after trip, got %v", err)
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected open, got %s", cb.State())
	}
}

func TestCircuitBreaker_ContextCanceledDoesNotCountAsFailure(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDuration:     1 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for i := 0; i < 5; i++ {
		err := cb.Call(ctx, func(ctx context.Context) error {
			return ctx.Err()
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("call %d should return context.Canceled, got %v", i, err)
		}
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected closed (context cancel not a failure), got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenProbeSucceeds(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold:  2,
		OpenDuration:      50 * time.Millisecond,
		HalfOpenMaxProbes: 1,
	})

	failErr := errors.New("down")
	for i := 0; i < 2; i++ {
		_ = cb.Call(context.Background(), func(ctx context.Context) error { return failErr })
	}
	if !errors.Is(cb.Call(context.Background(), func(ctx context.Context) error { return nil }), ErrCircuitOpen) {
		t.Fatal("should be open after failures")
	}

	time.Sleep(80 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Fatalf("expected half-open after sleep, got %s", cb.State())
	}

	err := cb.Call(context.Background(), func(ctx context.Context) error { return nil })
	if err != nil {
		t.Fatalf("probe should succeed, got %v", err)
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected closed after successful probe, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenProbeFails_Reopens(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold:  2,
		OpenDuration:      50 * time.Millisecond,
		HalfOpenMaxProbes: 1,
	})

	failErr := errors.New("still down")
	for i := 0; i < 2; i++ {
		_ = cb.Call(context.Background(), func(ctx context.Context) error { return failErr })
	}

	time.Sleep(80 * time.Millisecond)

	err := cb.Call(context.Background(), func(ctx context.Context) error { return failErr })
	if !errors.Is(err, failErr) {
		t.Fatalf("probe should return failErr, got %v", err)
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected reopened after failed probe, got %s", cb.State())
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		OpenDuration:     1 * time.Second,
	})
	failErr := errors.New("oops")

	_ = cb.Call(context.Background(), func(ctx context.Context) error { return failErr })
	_ = cb.Call(context.Background(), func(ctx context.Context) error { return failErr })
	_ = cb.Call(context.Background(), func(ctx context.Context) error { return nil })

	for i := 0; i < 2; i++ {
		err := cb.Call(context.Background(), func(ctx context.Context) error { return failErr })
		if !errors.Is(err, failErr) {
			t.Fatalf("expected failure, got %v", err)
		}
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected closed (success reset counter), got %s", cb.State())
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 100,
		OpenDuration:     time.Hour,
	})

	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = cb.Call(context.Background(), func(ctx context.Context) error { return nil })
			}
		}()
	}
	wg.Wait()

	if cb.State() != StateClosed {
		t.Fatalf("expected closed after concurrent successes, got %s", cb.State())
	}
}

func TestCircuitBreaker_DefaultConfigThreshold(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{OpenDuration: 100 * time.Millisecond})
	failErr := errors.New("fail")

	for i := 0; i < 5; i++ {
		err := cb.Call(context.Background(), func(ctx context.Context) error {
			return failErr
		})
		if !errors.Is(err, failErr) {
			t.Fatalf("call %d expected failErr, got %v", i, err)
		}
	}

	err := cb.Call(context.Background(), func(ctx context.Context) error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen at 5th failure with default threshold, got %v", err)
	}
}
