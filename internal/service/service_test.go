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

func newService(src *fakeSource, store Store, companies ...string) *Service {
	var boards []Board
	for _, c := range companies {
		boards = append(boards, Board{Source: src, Company: c})
	}
	return New(boards, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
