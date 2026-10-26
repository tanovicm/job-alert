package storage

import (
	"context"
	"testing"

	"job-alert/internal/domain"
)

func posting(id, title string) domain.Posting {
	return domain.Posting{Source: "greenhouse", Company: "acme", JobID: id, Title: title}
}

func save(t *testing.T, m *Memory, ps ...domain.Posting) []domain.Posting {
	t.Helper()
	fresh, err := m.Save(context.Background(), ps)
	if err != nil {
		t.Fatal(err)
	}
	return fresh
}

func list(t *testing.T, m *Memory) []domain.Posting {
	t.Helper()
	got, err := m.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSaveNewOnlyFirstTime(t *testing.T) {
	m := NewMemory()
	p := posting("1", "Go Engineer")

	if fresh := save(t, m, p); len(fresh) != 1 || fresh[0] != p {
		t.Fatalf("first save: new = %+v, want [%+v]", fresh, p)
	}
	if fresh := save(t, m, p); len(fresh) != 0 {
		t.Fatalf("second save: new = %+v, want none", fresh)
	}
}

func TestSaveEditedPostingIsNotNew(t *testing.T) {
	m := NewMemory()
	save(t, m, posting("1", "Go Engineer"))

	edited := posting("1", "Senior Go Engineer")
	if fresh := save(t, m, edited); len(fresh) != 0 {
		t.Fatalf("edited posting reported as new: %+v", fresh)
	}
	if got := list(t, m); len(got) != 1 || got[0].Title != "Senior Go Engineer" {
		t.Fatalf("edit not stored: %+v", got)
	}
}

func TestSaveNewJobIDIsNew(t *testing.T) {
	m := NewMemory()
	save(t, m, posting("1", "Go Engineer"))

	republished := posting("2", "Go Engineer")
	if fresh := save(t, m, posting("1", "Go Engineer"), republished); len(fresh) != 1 || fresh[0] != republished {
		t.Fatalf("new = %+v, want only [%+v]", fresh, republished)
	}
}

func TestSaveDuplicateInOneCall(t *testing.T) {
	m := NewMemory()
	p := posting("1", "Go Engineer")

	if fresh := save(t, m, p, p); len(fresh) != 1 {
		t.Fatalf("new = %+v, want the posting once", fresh)
	}
	if got := list(t, m); len(got) != 1 {
		t.Fatalf("stored %d postings, want 1: %+v", len(got), got)
	}
}

func TestListFirstSeenOrderWithoutDuplicates(t *testing.T) {
	m := NewMemory()
	save(t, m, posting("2", "Backend Engineer"), posting("1", "Go Engineer"))
	save(t, m, posting("3", "Platform Engineer"), posting("2", "Senior Backend Engineer"))

	got := list(t, m)
	want := []string{"2", "1", "3"}
	if len(got) != len(want) {
		t.Fatalf("listed %d postings, want %d: %+v", len(got), len(want), got)
	}
	for i, id := range want {
		if got[i].JobID != id {
			t.Errorf("position %d: job %s, want %s", i, got[i].JobID, id)
		}
	}
}
