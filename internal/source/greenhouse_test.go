package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"job-alert/internal/domain"
)

func TestGreenhouseFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/boards/acme/jobs" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"jobs":[
			{"id":123,"title":"Backend Engineer","absolute_url":"https://example.com/123","location":{"name":"Remote"}}
		]}`))
	}))
	defer srv.Close()

	g := &Greenhouse{BaseURL: srv.URL, Client: srv.Client()}

	got, err := g.Fetch(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Posting{
		Source:   "greenhouse",
		Company:  "acme",
		JobID:    "123",
		Title:    "Backend Engineer",
		Location: "Remote",
		URL:      "https://example.com/123",
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %+v, want [%+v]", got, want)
	}
}

func TestGreenhouseFetchErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	g := &Greenhouse{BaseURL: srv.URL, Client: srv.Client()}

	if _, err := g.Fetch(context.Background(), "missing"); err == nil {
		t.Fatal("expected an error for a 404 response")
	}
}
