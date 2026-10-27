package storage

import (
	"context"
	"os"
	"testing"

	"job-alert/internal/domain"
)

// These tests run against a real PostgreSQL database and are skipped unless
// TEST_DATABASE_URL is set. They empty the postings table.
func openTestPostgres(t *testing.T) *Postgres {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	p, err := NewPostgres(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	if err := p.db.Exec("TRUNCATE postings").Error; err != nil {
		t.Fatal(err)
	}
	return p
}

func pgSave(t *testing.T, p *Postgres, ps ...domain.Posting) []domain.Posting {
	t.Helper()
	fresh, err := p.Save(context.Background(), ps)
	if err != nil {
		t.Fatal(err)
	}
	return fresh
}

func pgList(t *testing.T, p *Postgres) []domain.Posting {
	t.Helper()
	got, err := p.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestPostgresSaveNewOnlyFirstTime(t *testing.T) {
	p := openTestPostgres(t)
	ps := posting("1", "Go Engineer")

	if fresh := pgSave(t, p, ps); len(fresh) != 1 || fresh[0] != ps {
		t.Fatalf("first save: new = %+v, want [%+v]", fresh, ps)
	}
	if fresh := pgSave(t, p, ps); len(fresh) != 0 {
		t.Fatalf("second save: new = %+v, want none", fresh)
	}
}

func TestPostgresSaveEditedPostingIsNotNew(t *testing.T) {
	p := openTestPostgres(t)
	pgSave(t, p, posting("1", "Go Engineer"))

	edited := posting("1", "Senior Go Engineer")
	if fresh := pgSave(t, p, edited); len(fresh) != 0 {
		t.Fatalf("edited posting reported as new: %+v", fresh)
	}
	if got := pgList(t, p); len(got) != 1 || got[0].Title != "Senior Go Engineer" {
		t.Fatalf("edit not stored: %+v", got)
	}
}

func TestPostgresSaveNewJobIDIsNew(t *testing.T) {
	p := openTestPostgres(t)
	pgSave(t, p, posting("1", "Go Engineer"))

	republished := posting("2", "Go Engineer")
	if fresh := pgSave(t, p, posting("1", "Go Engineer"), republished); len(fresh) != 1 || fresh[0] != republished {
		t.Fatalf("new = %+v, want only [%+v]", fresh, republished)
	}
}

func TestPostgresSaveDuplicateInOneCall(t *testing.T) {
	p := openTestPostgres(t)
	ps := posting("1", "Go Engineer")

	if fresh := pgSave(t, p, ps, ps); len(fresh) != 1 {
		t.Fatalf("new = %+v, want the posting once", fresh)
	}
	if got := pgList(t, p); len(got) != 1 {
		t.Fatalf("stored %d postings, want 1: %+v", len(got), got)
	}
}

func TestPostgresListFirstSeenOrder(t *testing.T) {
	p := openTestPostgres(t)
	pgSave(t, p, posting("2", "Backend Engineer"))
	pgSave(t, p, posting("1", "Go Engineer"))
	pgSave(t, p, posting("2", "Senior Backend Engineer")) // seen again, keeps its place

	got := pgList(t, p)
	if len(got) != 2 || got[0].JobID != "2" || got[1].JobID != "1" {
		t.Fatalf("list = %+v, want jobs 2 then 1", got)
	}
}

func TestPostgresSurvivesReconnect(t *testing.T) {
	p := openTestPostgres(t)
	ps := posting("1", "Go Engineer")
	pgSave(t, p, ps)
	p.Close()

	// A new connection, as after a service restart.
	again, err := NewPostgres(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()

	if got := pgList(t, again); len(got) != 1 || got[0] != ps {
		t.Fatalf("after reconnect: list = %+v, want [%+v]", got, ps)
	}
	if fresh := pgSave(t, again, ps); len(fresh) != 0 {
		t.Fatalf("after reconnect: %+v reported as new", fresh)
	}
}
