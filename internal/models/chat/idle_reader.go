package chat

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrIdleTimeout is returned by an IdleReader when no data arrives within the
// configured idle window. It signals a stalled LLM stream (e.g. 200 + one
// token + hang) that would otherwise block a goroutine indefinitely.
var ErrIdleTimeout = errors.New("llm stream idle timeout")

// idleReadGrace is the upper bound on how long a single IdleReader.Read call
// may block waiting for the underlying read before being considered idle. The
// per-reader idle timeout is the real bound; this constant exists only to
// document that the watchdog itself is bounded (the timer fires, the read
// goroutine is abandoned to GC).
const idleReadGrace = 0 // unused; kept for clarity

// IdleReader wraps an io.Reader so that every Read call is bounded by an idle
// timeout (and optionally a context). If the underlying reader does not
// produce data within the idle window, Read returns ErrIdleTimeout instead of
// blocking forever. This protects LLM streaming consumers from a stalled
// provider stream that sends a 200 + partial body then hangs.
//
// The abandoned underlying read goroutine is left to the runtime: the body is
// an HTTP response body whose Close (called by the stream processor's defer)
// unblocks the read. Callers MUST still Close the response body.
type IdleReader struct {
	src    io.Reader
	idle   time.Duration
	ctx    context.Context // cancelled on Close/stop
	cancel context.CancelFunc
}

// NewIdleReader wraps src with an idle timeout. A zero idle disables the
// watchdog (Read passes through). The reader is bound to context.Background.
func NewIdleReader(src io.Reader, idle time.Duration) *IdleReader {
	return NewIdleReaderContext(context.Background(), src, idle)
}

// NewIdleReaderContext is like NewIdleReader but the watchdog also honours ctx
// cancellation: if ctx is cancelled while a Read is blocked, Read returns the
// context's error promptly rather than waiting for the idle window.
func NewIdleReaderContext(ctx context.Context, src io.Reader, idle time.Duration) *IdleReader {
	if ctx == nil {
		ctx = context.Background()
	}
	cctx, cancel := context.WithCancel(ctx)
	return &IdleReader{src: src, idle: idle, ctx: cctx, cancel: cancel}
}

// Read implements io.Reader with an idle-timeout watchdog. It blocks until
// either the underlying reader returns data, the idle window elapses
// (ErrIdleTimeout), or the bound context is cancelled (ctx.Err()).
func (r *IdleReader) Read(p []byte) (int, error) {
	if r.idle <= 0 {
		// Watchdog disabled; still honour context cancellation.
		select {
		case <-r.ctx.Done():
			return 0, r.ctx.Err()
		default:
			return r.src.Read(p)
		}
	}

	type result struct {
		n   int
		err error
	}
	resCh := make(chan result, 1)

	go func() {
		n, err := r.src.Read(p)
		resCh <- result{n, err}
	}()

	timer := time.NewTimer(r.idle)
	defer timer.Stop()

	select {
	case res := <-resCh:
		return res.n, res.err
	case <-timer.C:
		// Stalled. Cancel so the context-cancel path below wins on subsequent
		// reads and signals the stream processor to stop.
		r.cancel()
		return 0, ErrIdleTimeout
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	}
}

// Close cancels the watchdog context. The underlying reader is NOT closed
// here (it is typically an http.Response body owned by the caller, who must
// Close it to unblock any abandoned read goroutine).
func (r *IdleReader) Close() error {
	r.cancel()
	return nil
}
