package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"job-alert/internal/domain"
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
	var boards []Board
	for _, c := range companies {
		boards = append(boards, Board{Source: src, Company: c})
	}
	return New(boards, store, filter, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
