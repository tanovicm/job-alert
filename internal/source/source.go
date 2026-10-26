// Package source defines the adapter interface for job boards and its implementations.
package source

import (
	"context"

	"job-alert/internal/domain"
)

// Source fetches open postings for one company from one job board.
// Adding a new board means adding a new implementation of this interface.
type Source interface {
	// Name identifies the board, e.g. "greenhouse".
	Name() string
	// Fetch returns all currently open postings for the given company.
	Fetch(ctx context.Context, company string) ([]domain.Posting, error)
}
