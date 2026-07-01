package chat

import (
	"context"
	"errors"
	"sync"
	"time"
)

type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "closed"
	StateOpen     CircuitBreakerState = "open"
	StateHalfOpen CircuitBreakerState = "half_open"
)

type CircuitBreakerConfig struct {
	FailureThreshold int
	OpenDuration     time.Duration
	HalfOpenMaxProbes int
	IsFailure        func(err error) bool
}

func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:  5,
		OpenDuration:      30 * time.Second,
		HalfOpenMaxProbes: 1,
	}
}

var ErrCircuitOpen = errors.New("circuit breaker open: model provider temporarily unavailable")

type CircuitBreaker struct {
	cfg CircuitBreakerConfig
	mu  sync.Mutex

	state         CircuitBreakerState
	failures      int
	openedAt      time.Time
	halfOpenProbe int
}

func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.OpenDuration <= 0 {
		cfg.OpenDuration = 30 * time.Second
	}
	if cfg.HalfOpenMaxProbes <= 0 {
		cfg.HalfOpenMaxProbes = 1
	}
	if cfg.IsFailure == nil {
		cfg.IsFailure = func(err error) bool { return err != nil }
	}
	return &CircuitBreaker{cfg: cfg, state: StateClosed}
}

func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.maybeTransitionLocked()
	return cb.state
}

func (cb *CircuitBreaker) Call(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := cb.allow(); err != nil {
		return err
	}
	err := fn(ctx)
	isContextCanceled := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
	if isContextCanceled {
		cb.record(true, nil)
	} else {
		cb.record(err == nil, err)
	}
	return err
}

func (cb *CircuitBreaker) allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.maybeTransitionLocked()
	switch cb.state {
	case StateOpen:
		return ErrCircuitOpen
	case StateHalfOpen:
		if cb.halfOpenProbe >= cb.cfg.HalfOpenMaxProbes {
			return ErrCircuitOpen
		}
		cb.halfOpenProbe++
		return nil
	default:
		return nil
	}
}

func (cb *CircuitBreaker) record(success bool, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	isFailure := !success
	if !isFailure && err != nil && cb.cfg.IsFailure(err) {
		isFailure = true
	}

	switch cb.state {
	case StateHalfOpen:
		if isFailure {
			cb.tripLocked()
			return
		}
		if cb.halfOpenProbe >= cb.cfg.HalfOpenMaxProbes {
			cb.closeLocked()
		}
	case StateClosed:
		if isFailure {
			cb.failures++
			if cb.failures >= cb.cfg.FailureThreshold {
				cb.tripLocked()
			}
		} else {
			cb.failures = 0
		}
	}
}

func (cb *CircuitBreaker) maybeTransitionLocked() {
	switch cb.state {
	case StateOpen:
		if time.Since(cb.openedAt) >= cb.cfg.OpenDuration {
			cb.state = StateHalfOpen
			cb.halfOpenProbe = 0
		}
	}
}

func (cb *CircuitBreaker) tripLocked() {
	cb.state = StateOpen
	cb.openedAt = time.Now()
	cb.failures = 0
}

func (cb *CircuitBreaker) closeLocked() {
	cb.state = StateClosed
	cb.failures = 0
	cb.halfOpenProbe = 0
}
