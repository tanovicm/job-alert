// Package service contains the business logic: checking job boards for postings.
package service

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"

	"job-alert/internal/domain"
	"job-alert/internal/source"
)

// ErrRunInProgress is returned when a run is requested while one is running.
var ErrRunInProgress = errors.New("a run is already in progress")

// Store persists postings.
type Store interface {
	// Save stores postings and returns only the ones that were not stored before.
	Save(ctx context.Context, postings []domain.Posting) ([]domain.Posting, error)
	// List returns all stored postings.
	List(ctx context.Context) ([]domain.Posting, error)
}

// Filter decides whether a posting is relevant.
type Filter interface {
	Match(p domain.Posting) bool
}

// Board is one company on one job board.
type Board struct {
	Source  source.Source
	Company string
}

// Service checks boards and stores what it finds. Only one run executes at a time.
type Service struct {
	boards  []Board
	store   Store
	filter  Filter
	log     *slog.Logger
	running atomic.Bool
}

func New(boards []Board, store Store, filter Filter, log *slog.Logger) *Service {
	return &Service{boards: boards, store: store, filter: filter, log: log}
}

// StartRun starts a run in the background and returns immediately.
// It returns ErrRunInProgress if a run is already running.
func (s *Service) StartRun(ctx context.Context) error {
	if !s.running.CompareAndSwap(false, true) {
		return ErrRunInProgress
	}
	go func() {
		defer s.running.Store(false)
		s.check(ctx)
	}()
	return nil
}

// Run checks every board once and waits for it to finish.
// It returns ErrRunInProgress if a run is already running.
func (s *Service) Run(ctx context.Context) error {
	if !s.running.CompareAndSwap(false, true) {
		return ErrRunInProgress
	}
	defer s.running.Store(false)
	s.check(ctx)
	return nil
}

// check fetches postings for every board and stores the relevant ones.
// A failing board is logged and skipped so the others still get checked.
func (s *Service) check(ctx context.Context) {
	for _, b := range s.boards {
		postings, err := b.Source.Fetch(ctx, b.Company)
		if err != nil {
			s.log.Error("fetch failed", "source", b.Source.Name(), "company", b.Company, "err", err)
			continue
		}

		var relevant []domain.Posting
		for _, p := range postings {
			if s.filter.Match(p) {
				relevant = append(relevant, p)
			}
		}

		fresh, err := s.store.Save(ctx, relevant)
		if err != nil {
			s.log.Error("save failed", "source", b.Source.Name(), "company", b.Company, "err", err)
			continue
		}
		s.log.Info("board checked", "source", b.Source.Name(), "company", b.Company,
			"fetched", len(postings), "relevant", len(relevant), "new", len(fresh))
	}
}

// ListPostings returns the stored postings.
func (s *Service) ListPostings(ctx context.Context) ([]domain.Posting, error) {
	return s.store.List(ctx)
}
