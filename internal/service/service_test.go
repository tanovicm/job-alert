package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"job-alert/internal/domain"
	"job-alert/internal/source"
	"job-alert/internal/storage"
)

// fakeSource returns fixed postings per company, or an error.
type fakeSource struct {
	postings map[string][]domain.Posting
	errs     map[string]error
	block    chan struct{} // if set, Fetch waits until it is closed
}

func (f *fakeSource) Name() string { return "fake" }

func (f *fakeSource) Fetch(_ context.Context, company string) ([]domain.Posting, error) {
	if f.block != nil {
		<-f.block
	}
	if err := f.errs[company]; err != nil {
		return nil, err
	}
	return f.postings[company], nil
}

// matchAll lets every posting through.
type matchAll struct{}

func (matchAll) Match(domain.Posting) bool { return true }

// matchTitles lets through only postings with one of the given titles.
type matchTitles map[string]bool

func (m matchTitles) Match(p domain.Posting) bool { return m[p.Title] }

func newService(src *fakeSource, store Store, companies ...string) *Service {
	return newFilteredService(src, store, matchAll{}, companies...)
}

func newFilteredService(src *fakeSource, store Store, filter Filter, companies ...string) *Service {
	return New(boards(src, companies...), store, filter, 5, discard)
}

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func boards(src source.Source, companies ...string) []Board {
	var bs []Board
	for _, c := range companies {
		bs = append(bs, Board{Source: src, Company: c})
	}
	return bs
}

func companies(n int) []string {
	var cs []string
	for i := range n {
		cs = append(cs, fmt.Sprintf("company-%d", i))
	}
	return cs
}

// trackingSource records how many Fetch calls run at the same time. Each call
// takes delay, or until ctx is cancelled, so that calls overlap.
type trackingSource struct {
	delay time.Duration

	inFlight atomic.Int32
	maxSeen  atomic.Int32
	started  atomic.Int32
	finished atomic.Int32
}

func (s *trackingSource) Name() string { return "tracking" }

func (s *trackingSource) Fetch(ctx context.Context, company string) ([]domain.Posting, error) {
	s.started.Add(1)
	defer s.finished.Add(1)

	n := s.inFlight.Add(1)
	defer s.inFlight.Add(-1)
	for {
		m := s.maxSeen.Load()
		if n <= m || s.maxSeen.CompareAndSwap(m, n) {
			break
		}
	}

	select {
	case <-time.After(s.delay):
		return []domain.Posting{posting(company, "1", "Go Engineer")}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestRunLimitsConcurrency(t *testing.T) {
	src := &trackingSource{delay: 20 * time.Millisecond}
	svc := New(boards(src, companies(12)...), storage.NewMemory(), matchAll{}, 3, discard)

	if err := svc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := src.maxSeen.Load(); got != 3 {
		t.Errorf("at most %d boards ran at the same time, want exactly the limit of 3", got)
	}
	// Run must not return before every board is done.
	if got := src.finished.Load(); got != 12 {
		t.Errorf("%d of 12 boards finished when Run returned", got)
	}
}

func TestRunWithConcurrencyOneIsSequential(t *testing.T) {
	src := &trackingSource{delay: 5 * time.Millisecond}
	svc := New(boards(src, companies(4)...), storage.NewMemory(), matchAll{}, 1, discard)

	if err := svc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := src.maxSeen.Load(); got != 1 {
		t.Errorf("%d boards ran at the same time, want 1", got)
	}
}

func TestRunStopsStartingBoardsWhenCancelled(t *testing.T) {
	src := &trackingSource{delay: time.Hour} // only cancellation ends a Fetch
	svc := New(boards(src, companies(10)...), storage.NewMemory(), matchAll{}, 2, discard)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.Run(ctx)
		close(done)
	}()

	// Wait until the first two boards are in flight, then cancel.
	deadline := time.Now().Add(time.Second)
	for src.inFlight.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancellation")
	}
	if got := src.started.Load(); got != 2 {
		t.Errorf("%d boards started, want only the 2 in flight before cancellation", got)
	}
	if got := src.inFlight.Load(); got != 0 {
		t.Errorf("%d fetches still running after Run returned", got)
	}
}

func posting(company, id, title string) domain.Posting {
	return domain.Posting{Source: "fake", Company: company, JobID: id, Title: title}
}

func TestRunStoresPostingsWithoutDuplicates(t *testing.T) {
	ctx := context.Background()
	src := &fakeSource{postings: map[string][]domain.Posting{
		"acme": {posting("acme", "1", "Go Engineer")},
	}}
	store := storage.NewMemory()
	svc := newService(src, store, "acme")

	if err := svc.Run(ctx); err != nil {
		t.Fatal(err)
	}

	// Second run: job 1 is edited (same ID), job 2 is new.
	src.postings["acme"] = []domain.Posting{
		posting("acme", "1", "Senior Go Engineer"),
		posting("acme", "2", "Backend Engineer"),
	}
	if err := svc.Run(ctx); err != nil {
		t.Fatal(err)
	}

	got, err := svc.ListPostings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("stored %d postings, want 2: %+v", len(got), got)
	}
	if got[0].Title != "Senior Go Engineer" {
		t.Errorf("edit not stored: title = %q", got[0].Title)
	}
}

func TestRunStoresOnlyRelevantPostings(t *testing.T) {
	ctx := context.Background()
	src := &fakeSource{postings: map[string][]domain.Posting{
		"acme": {
			posting("acme", "1", "Go Engineer"),
			posting("acme", "2", "Account Executive"),
			posting("acme", "3", "Backend Engineer"),
		},
	}}
	filter := matchTitles{"Go Engineer": true, "Backend Engineer": true}
	svc := newFilteredService(src, storage.NewMemory(), filter, "acme")

	if err := svc.Run(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ListPostings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].JobID != "1" || got[1].JobID != "3" {
		t.Fatalf("stored %+v, want only jobs 1 and 3", got)
	}
}

func TestRunContinuesPastFailingBoard(t *testing.T) {
	ctx := context.Background()
	src := &fakeSource{
		postings: map[string][]domain.Posting{"good": {posting("good", "1", "Go Engineer")}},
		errs:     map[string]error{"bad": errors.New("boom")},
	}
	svc := newService(src, storage.NewMemory(), "bad", "good")

	if err := svc.Run(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.ListPostings(ctx)
	if len(got) != 1 {
		t.Fatalf("stored %d postings, want 1 from the working board", len(got))
	}
}

func TestStartRunRejectsOverlappingRun(t *testing.T) {
	ctx := context.Background()
	src := &fakeSource{block: make(chan struct{})}
	svc := newService(src, storage.NewMemory(), "acme")

	if err := svc.StartRun(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartRun(ctx); !errors.Is(err, ErrRunInProgress) {
		t.Fatalf("second StartRun: err = %v, want ErrRunInProgress", err)
	}
	if err := svc.Run(ctx); !errors.Is(err, ErrRunInProgress) {
		t.Fatalf("Run during StartRun: err = %v, want ErrRunInProgress", err)
	}
	close(src.block)
}
