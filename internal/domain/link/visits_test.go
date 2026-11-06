package link

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// countingRepo is a THREAD-SAFE fake. fakeRepo from service_test.go has plain int counters:
// with 4 workers writing to them, `go test -race` would (rightly) fail.
//
// The embedded nil Repository is a Go trick: countingRepo "has" every method of the
// interface, but only RecordVisit is real. Calling any other one panics on the nil,
// which is what we want, because the recorder must never call anything else.
type countingRepo struct {
	Repository
	mu     sync.Mutex
	counts map[string]int
	block  chan struct{} // if non-nil, RecordVisit waits until it is closed: simulates a slow database
}

func newCountingRepo() *countingRepo {
	return &countingRepo{counts: map[string]int{}}
}

func (c *countingRepo) RecordVisit(ctx context.Context, code string) error {
	if c.block != nil {
		<-c.block
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[code]++
	return nil
}

func (c *countingRepo) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.counts {
		n += v
	}
	return n
}

func closeWithin(t *testing.T, r *AsyncRecorder, d time.Duration) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return r.Close(ctx)
}

// The central promise: parallel callers, nothing lost. Run it with -race.
func TestAsyncRecorder_NoVisitLostUnderConcurrency(t *testing.T) {
	const callers, perCaller = 50, 100
	repo := newCountingRepo()
	rec := NewAsyncRecorder(repo, 4, callers*perCaller) // queue big enough to never drop here

	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perCaller; j++ {
				rec.Record("abc123")
			}
		}()
	}
	wg.Wait() // every Record call has returned...

	if err := closeWithin(t, rec, 5*time.Second); err != nil { // ...and Close waits for the workers to drain
		t.Fatalf("Close: %v", err)
	}
	if got := repo.total(); got != callers*perCaller {
		t.Errorf("counted %d visits, want %d", got, callers*perCaller)
	}
	if rec.Dropped() != 0 {
		t.Errorf("dropped %d, want 0", rec.Dropped())
	}
}

func TestAsyncRecorder_DropsWhenQueueIsFull(t *testing.T) {
	repo := newCountingRepo()
	repo.block = make(chan struct{}) // the only worker is stuck on its first visit
	rec := NewAsyncRecorder(repo, 1, 2)

	// Capacity is 1 (in the worker's hands) + 2 (queue) = 3. The other 7 must be dropped.
	// The test must not hang: Record never blocks, even though nothing is being consumed.
	start := time.Now()
	for i := 0; i < 10; i++ {
		rec.Record("abc123")
	}
	if time.Since(start) > time.Second {
		t.Fatal("Record blocked on a full queue")
	}
	if rec.Dropped() < 7 {
		t.Errorf("dropped = %d, want at least 7", rec.Dropped())
	}

	close(repo.block) // unstick the worker
	if err := closeWithin(t, rec, 5*time.Second); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Nothing disappears silently: every visit is either counted or counted as dropped.
	if got := repo.total() + int(rec.Dropped()); got != 10 {
		t.Errorf("counted + dropped = %d, want 10", got)
	}
}

func TestAsyncRecorder_CloseDrainsTheQueue(t *testing.T) {
	repo := newCountingRepo()
	rec := NewAsyncRecorder(repo, 2, 100)
	for i := 0; i < 20; i++ {
		rec.Record("abc123")
	}

	if err := closeWithin(t, rec, 5*time.Second); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := repo.total(); got != 20 {
		t.Errorf("counted %d, want 20: Close must finish the queued work, not discard it", got)
	}
}

// Without the RWMutex this would panic: "send on closed channel".
func TestAsyncRecorder_RecordAfterCloseDoesNotPanic(t *testing.T) {
	repo := newCountingRepo()
	rec := NewAsyncRecorder(repo, 1, 10)
	if err := closeWithin(t, rec, time.Second); err != nil {
		t.Fatalf("Close: %v", err)
	}

	rec.Record("abc123") // must be ignored quietly

	if got := repo.total(); got != 0 {
		t.Errorf("counted %d after Close, want 0", got)
	}
	if err := closeWithin(t, rec, time.Second); err != nil { // a second Close must be harmless
		t.Errorf("second Close: %v", err)
	}
}

func TestAsyncRecorder_CloseGivesUpAtTheDeadline(t *testing.T) {
	repo := newCountingRepo()
	repo.block = make(chan struct{})
	rec := NewAsyncRecorder(repo, 1, 10)
	rec.Record("abc123") // the worker is now stuck forever
	defer close(repo.block)

	err := closeWithin(t, rec, 50*time.Millisecond)

	if err != context.DeadlineExceeded {
		t.Errorf("Close = %v, want context.DeadlineExceeded: a stuck worker must not hang shutdown", err)
	}
}

type panicRepo struct{ Repository }

func (panicRepo) RecordVisit(context.Context, string) error { panic("bad row") }

func TestAsyncRecorder_SurvivesAPanicInTheRepository(t *testing.T) {
	rec := NewAsyncRecorder(panicRepo{}, 1, 10)
	rec.Record("abc123")
	rec.Record("abc123")

	// If the panic were not recovered, this test binary would crash before reaching here.
	if err := closeWithin(t, rec, 5*time.Second); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

type fakeRecorder struct{ codes []string }

func (f *fakeRecorder) Record(code string) { f.codes = append(f.codes, code) }

func TestService_Resolve_DelegatesCountingToTheRecorder(t *testing.T) {
	repo := &fakeRepo{findLink: Link{ShortCode: "abc123", LongURL: "https://example.com"}}
	rec := &fakeRecorder{}

	_, err := NewService(repo, WithVisitRecorder(rec)).Resolve(context.Background(), "abc123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.codes) != 1 || rec.codes[0] != "abc123" {
		t.Errorf("recorder got %v, want [abc123]", rec.codes)
	}
	if repo.visitCalls != 0 {
		t.Errorf("service called RecordVisit itself %d times, it must go through the recorder", repo.visitCalls)
	}
}

// A missing link must not produce a visit: no count for something that does not exist.
func TestService_Resolve_NoVisitWhenNotFound(t *testing.T) {
	repo := &fakeRepo{findErr: ErrNotFound}
	rec := &fakeRecorder{}

	_, err := NewService(repo, WithVisitRecorder(rec)).Resolve(context.Background(), "abc123")

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if len(rec.codes) != 0 {
		t.Errorf("recorder got %v, want nothing", rec.codes)
	}
}
