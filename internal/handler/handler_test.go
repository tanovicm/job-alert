package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"job-alert/internal/domain"
	"job-alert/internal/service"
)

type fakeService struct {
	startErr error
	postings []domain.Posting
}

func (f *fakeService) StartRun(context.Context) error { return f.startErr }

func (f *fakeService) ListPostings(context.Context) ([]domain.Posting, error) {
	return f.postings, nil
}

func serve(svc Service, method, path string) *httptest.ResponseRecorder {
	h := New(context.Background(), svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestPostRuns(t *testing.T) {
	tests := []struct {
		name     string
		startErr error
		want     int
	}{
		{"accepted", nil, http.StatusAccepted},
		{"run in progress", service.ErrRunInProgress, http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(&fakeService{startErr: tt.startErr}, http.MethodPost, "/runs")
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestGetPostings(t *testing.T) {
	svc := &fakeService{postings: []domain.Posting{
		{Source: "greenhouse", Company: "acme", JobID: "1", Title: "Go Engineer"},
	}}

	rec := serve(svc, http.MethodGet, "/postings")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Postings []domain.Posting `json:"postings"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Postings) != 1 || body.Postings[0].JobID != "1" {
		t.Errorf("postings = %+v", body.Postings)
	}
}
