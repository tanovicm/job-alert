package scheduler

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"job-alert/internal/service"
)

// fakeStarter records each StartRun call on a channel and returns err.
type fakeStarter struct {
	calls chan struct{}
	err   error
}

func newFakeStarter(err error) *fakeStarter {
	return &fakeStarter{calls: make(chan struct{}, 100), err: err}
}

func (f *fakeStarter) StartRun(context.Context) error {
	f.calls <- struct{}{}
	return f.err
}

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// waitCalls fails the test unless n calls arrive within timeout.
func waitCalls(t *testing.T, f *fakeStarter, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for i := 0; i < n; i++ {
		select {
		case <-f.calls:
		case <-deadline:
			t.Fatalf("got %d runs within %v, want %d", i, timeout, n)
		}
	}
}

func TestRunStartsImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newFakeStarter(nil)

	// With an hour-long interval, only the run on start can happen in time.
	go Run(ctx, f, time.Hour, discard)

	waitCalls(t, f, 1, time.Second)
}

func TestRunRepeatsEveryInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newFakeStarter(nil)

	go Run(ctx, f, 10*time.Millisecond, discard)

	waitCalls(t, f, 3, time.Second) // on start + two ticks
}

func TestRunKeepsGoingWhenRunInProgress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newFakeStarter(service.ErrRunInProgress)

	go Run(ctx, f, 10*time.Millisecond, discard)

	waitCalls(t, f, 3, time.Second)
}

func TestRunStopsWhenContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	f := newFakeStarter(nil)

	done := make(chan struct{})
	go func() {
		Run(ctx, f, 10*time.Millisecond, discard)
		close(done)
	}()
	waitCalls(t, f, 1, time.Second)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
}
