package chat

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// TestIdleReader_NonBlockingUnderActiveStream: when the underlying reader
// produces data promptly, IdleReader forwards the events without an idle
// timeout error (EOF at end-of-stream is the normal terminator, not a fault).
func TestIdleReader_NonBlockingUnderActiveStream(t *testing.T) {
	src := strings.NewReader("data: {\"type\":\"message_start\"}\n\ndata: {\"type\":\"message_stop\"}\n\n")
	r := NewIdleReader(src, 200*time.Millisecond)
	sse := NewSSEReader(r)

	count := 0
	for {
		ev, err := sse.ReadEvent()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("unexpected non-EOF error: %v", err)
		}
		count++
		if ev.Done {
			break
		}
	}
	if count < 2 {
		t.Fatalf("expected at least 2 events, got %d", count)
	}
}

// TestIdleReader_StalledStreamTimesOut: when the underlying reader blocks
// forever (never produces data), IdleReader surfaces ErrIdleTimeout after the
// idle window rather than blocking indefinitely.
func TestIdleReader_StalledStreamTimesOut(t *testing.T) {
	src := &stallingReader{}
	r := NewIdleReader(src, 50*time.Millisecond)
	sse := NewSSEReader(r)

	start := time.Now()
	_, err := sse.ReadEvent()
	elapsed := time.Since(start)
	if !errors.Is(err, ErrIdleTimeout) {
		t.Fatalf("expected ErrIdleTimeout, got %v (elapsed=%v)", err, elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("idle timeout fired too late: %v", elapsed)
	}
	if elapsed < 40*time.Millisecond {
		t.Fatalf("idle timeout fired too early: %v", elapsed)
	}
}

// TestIdleReader_ContextCancelAborts: when the supplied context is cancelled,
// the idle reader aborts promptly with the context error rather than waiting
// for the idle window.
func TestIdleReader_ContextCancelAborts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	src := &stallingReader{}
	r := NewIdleReaderContext(ctx, src, 5*time.Second)
	sse := NewSSEReader(r)

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := sse.ReadEvent()
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v (elapsed=%v)", err, elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("cancel took too long: %v", elapsed)
	}
}

// TestIdleReader_MidStreamStallTimesOut: a stream that delivers one event then
// stalls still triggers the idle timeout on the SECOND read.
func TestIdleReader_MidStreamStallTimesOut(t *testing.T) {
	src := strings.NewReader("data: {\"type\":\"message_start\"}\n\n")
	r := NewIdleReader(src, 50*time.Millisecond)
	sse := NewSSEReader(r)

	// First read succeeds promptly.
	ev, err := sse.ReadEvent()
	if err != nil || ev == nil {
		t.Fatalf("first read should succeed, err=%v ev=%v", err, ev)
	}
	// Second read hits EOF (source exhausted), not a timeout. To test a true
	// mid-stream stall we need a reader that stalls after the first event.
	stallSrc := &stallAfterOneReader{first: "data: {\"type\":\"message_start\"}\n\n"}
	r2 := NewIdleReader(stallSrc, 50*time.Millisecond)
	sse2 := NewSSEReader(r2)

	ev, err = sse2.ReadEvent()
	if err != nil || ev == nil {
		t.Fatalf("first read should succeed, err=%v", err)
	}
	_, err = sse2.ReadEvent()
	if !errors.Is(err, ErrIdleTimeout) {
		t.Fatalf("second read should time out, got %v", err)
	}
}

// stallingReader never produces data and never returns EOF.
type stallingReader struct{}

func (stallingReader) Read(p []byte) (int, error) {
	select {} // block forever
}

// stallAfterOneReader returns one event then blocks forever.
type stallAfterOneReader struct {
	first string
	done  bool
}

func (r *stallAfterOneReader) Read(p []byte) (int, error) {
	if !r.done {
		r.done = true
		copy(p, r.first)
		return len(r.first), nil
	}
	select {} // block forever
}

// Ensure io.EOF sentinel is available for the implementation.
var _ = io.EOF
