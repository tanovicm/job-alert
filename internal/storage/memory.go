// Package storage provides Store implementations.
package storage

import (
	"context"
	"sync"

	"job-alert/internal/domain"
)

// Memory is an in-memory store. It does not survive restarts; it stands in
// until the PostgreSQL store exists.
type Memory struct {
	mu       sync.Mutex
	postings map[domain.Key]domain.Posting
	order    []domain.Key // keys in the order first saved, so List is stable
}

func NewMemory() *Memory {
	return &Memory{postings: make(map[domain.Key]domain.Posting)}
}

// Save stores postings and returns only the ones that were not stored before.
// A posting that already exists is updated in place and not returned.
func (m *Memory) Save(_ context.Context, postings []domain.Posting) ([]domain.Posting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var fresh []domain.Posting
	for _, p := range postings {
		key := p.Key()
		if _, ok := m.postings[key]; !ok {
			m.order = append(m.order, key)
			fresh = append(fresh, p)
		}
		m.postings[key] = p
	}
	return fresh, nil
}

// List returns all stored postings in the order they were first saved.
func (m *Memory) List(_ context.Context) ([]domain.Posting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]domain.Posting, 0, len(m.order))
	for _, key := range m.order {
		out = append(out, m.postings[key])
	}
	return out, nil
}
