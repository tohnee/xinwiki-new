package chat

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned by CircuitBreaker.Call when the circuit is open
// (the wrapped call is NOT invoked). Callers should treat this as a
// non-retryable "provider temporarily unavailable" signal distinct from a real
// provider error.
var ErrCircuitOpen = errors.New("llm circuit breaker open")

// CircuitState enumerates the three breaker states.
type CircuitState int

const (
	// CircuitClosed: calls pass through; failures are counted.
	CircuitClosed CircuitState = iota
	// CircuitOpen: calls short-circuit with ErrCircuitOpen without invoking
	// the wrapped function.
	CircuitOpen
	// CircuitHalfOpen: one probe call is allowed to test recovery.
	CircuitHalfOpen
)

// CircuitBreakerConfig tunes a CircuitBreaker. Zero values are not valid;
// always construct via NewCircuitBreaker.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures that trips the
	// breaker from closed to open.
	FailureThreshold int
	// OpenDuration is how long the breaker stays open before transitioning to
	// half-open (allowing a single probe).
	OpenDuration time.Duration
}

// CircuitBreaker protects an LLM provider from cascading failures: after
// FailureThreshold consecutive failures it opens, fast-failing subsequent
// calls without hitting the provider; after OpenDuration it half-opens for a
// single probe. A nil *CircuitBreaker is a no-op (Call passes through) so the
// breaker is opt-in per provider.
//
// A context cancellation (ctx.Err) is intentionally NOT counted as a failure:
// a client-driven cancel is not a provider health signal and must not trip a
// breaker that other callers depend on.
type CircuitBreaker struct {
	cfg CircuitBreakerConfig

	mu               sync.Mutex
	state            CircuitState
	consecutiveFails int
	openedAt         time.Time
	// now is an injection seam so tests can advance time without sleeping; nil
	// means use wall-clock.
	now func() time.Time
}

// NewCircuitBreaker builds a breaker in the closed state.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{cfg: cfg, state: CircuitClosed, now: time.Now}
}

// Call invokes fn under the breaker's protection. Returns ErrCircuitOpen
// (without calling fn) when the breaker is open. A nil receiver passes fn
// through unchanged.
func (cb *CircuitBreaker) Call(ctx context.Context, fn func(ctx context.Context) error) error {
	if cb == nil {
		return fn(ctx)
	}

	// Check the gate before invoking. Half-open allows exactly one probe.
	if !cb.allowCall() {
		return ErrCircuitOpen
	}

	err := fn(ctx)

	// Context cancellation is the caller's choice, not a provider failure.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// Do not count toward the failure threshold; do not change state.
		return err
	}

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
	return err
}

// State returns the current breaker state (closed/open/half-open). Safe for
// concurrent use; primarily for observability and tests.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.refreshLocked()
	return cb.state
}

// allowCall reports whether a call may proceed and, if so, marks the probe
// in-flight for the half-open transition.
func (cb *CircuitBreaker) allowCall() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.refreshLocked()
	switch cb.state {
	case CircuitOpen:
		return false
	case CircuitHalfOpen:
		// Allow exactly one probe: transition back to open pessimistically so a
		// concurrent second caller is blocked until the probe resolves. The
		// probe's outcome will close or keep-open in onSuccess/onFailure.
		cb.state = CircuitOpen
		cb.openedAt = cb.now()
		return true
	default: // closed
		return true
	}
}

// onSuccess resets the failure counter and closes the circuit.
func (cb *CircuitBreaker) onSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFails = 0
	cb.state = CircuitClosed
}

// onFailure bumps the counter and trips the breaker when the threshold is
// reached.
func (cb *CircuitBreaker) onFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFails++
	if cb.consecutiveFails >= cb.cfg.FailureThreshold {
		cb.state = CircuitOpen
		cb.openedAt = cb.now()
	}
}

// refreshLocked transitions open -> half-open once OpenDuration has elapsed.
// Caller must hold cb.mu.
func (cb *CircuitBreaker) refreshLocked() {
	if cb.state != CircuitOpen {
		return
	}
	if cb.now().Sub(cb.openedAt) >= cb.cfg.OpenDuration {
		cb.state = CircuitHalfOpen
	}
}
