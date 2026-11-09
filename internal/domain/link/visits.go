package link

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// VisitRecorder is "fire and forget": the caller is never told whether the visit was
// stored. That is the whole point: a counter must not be able to slow down or break a redirect.
type VisitRecorder interface {
	Record(code string)
}

// visitTimeout bounds one counting operation. It replaces the request deadline,
// which no longer applies once the work leaves the request.
const visitTimeout = 3 * time.Second

// ---------- synchronous recorder (the default) ----------

// syncRecorder counts inline. It keeps the old behavior, so NewService(repo) still works
// and every existing test is unchanged.
type syncRecorder struct{ repo Repository }

func (s syncRecorder) Record(code string) {
	ctx, cancel := context.WithTimeout(context.Background(), visitTimeout)
	defer cancel()
	if err := s.repo.RecordVisit(ctx, code); err != nil {
		slog.Warn("record visit failed", "code", code, "error", err)
	}
}

// ---------- asynchronous recorder: queue + worker pool ----------

// AsyncRecorder accepts visits instantly and counts them on a fixed set of workers.
type AsyncRecorder struct {
	repo  Repository
	queue chan string // buffered: the "waiting room" between requests and workers
	wg    sync.WaitGroup

	// mu protects `closed` and, above all, the CLOSING of the channel.
	// Sending on a closed channel panics, so "check closed, then send" must be atomic
	// with respect to Close. Record takes the read lock (many at once), Close the write lock.
	mu     sync.RWMutex
	closed bool

	dropped atomic.Int64 // lock-free counter: many goroutines add to it at once
}

// NewAsyncRecorder starts `workers` goroutines. The queue holds up to `queueSize` visits.
func NewAsyncRecorder(repo Repository, workers, queueSize int) *AsyncRecorder {
	r := &AsyncRecorder{repo: repo, queue: make(chan string, queueSize)}
	r.wg.Add(workers) // BEFORE starting the goroutines, never inside them: Close could race it
	for i := 0; i < workers; i++ {
		go r.worker()
	}
	return r
}

// worker reads until the channel is closed AND empty. `range` over a channel does exactly that,
// which is what makes "close, then wait" a clean drain.
func (r *AsyncRecorder) worker() {
	defer r.wg.Done()
	for code := range r.queue {
		r.record(code)
	}
}

func (r *AsyncRecorder) record(code string) {
	// A panic in a goroutine that is not a request handler is NOT caught by the Recovery
	// middleware: it would crash the whole process. One bad row must not kill the server.
	defer func() {
		if p := recover(); p != nil {
			slog.Error("panic while recording visit", "code", code, "panic", p, "stack", string(debug.Stack()))
		}
	}()

	// context.Background, NOT the request's context: the request is already finished
	// (the user got the redirect), so its context is cancelled. Using it would cancel every count.
	ctx, cancel := context.WithTimeout(context.Background(), visitTimeout)
	defer cancel()
	if err := r.repo.RecordVisit(ctx, code); err != nil {
		slog.Warn("record visit failed", "code", code, "error", err)
	}
}

// Record never blocks.
func (r *AsyncRecorder) Record(code string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		slog.Warn("visit ignored: recorder is closed", "code", code)
		return
	}

	// select with a default is a NON-blocking send: if the queue has room we enqueue,
	// otherwise we fall to default immediately instead of waiting.
	select {
	case r.queue <- code:
	default:
		n := r.dropped.Add(1)
		slog.Warn("visit queue full, visit dropped", "code", code, "dropped_total", n)
	}
}

// Dropped returns how many visits were discarded because the queue was full.
func (r *AsyncRecorder) Dropped() int64 { return r.dropped.Load() }

// Close stops accepting visits, lets the workers finish everything already queued,
// and returns when they are done or when ctx expires, whichever comes first.
func (r *AsyncRecorder) Close(ctx context.Context) error {
	r.mu.Lock()
	if !r.closed {
		r.closed = true
		close(r.queue) // tells every worker's `range` loop: "after the remaining items, stop"
	}
	r.mu.Unlock()

	// wg.Wait cannot be given a deadline, so we wait in a goroutine and race it against ctx.
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
