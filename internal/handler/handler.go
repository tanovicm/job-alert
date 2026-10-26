// Package handler exposes the HTTP API: POST /runs and GET /postings.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"job-alert/internal/domain"
	"job-alert/internal/service"
)

// Service is what the handlers need from the service layer.
type Service interface {
	StartRun(ctx context.Context) error
	ListPostings(ctx context.Context) ([]domain.Posting, error)
}

type handler struct {
	// runCtx is used for background runs. It outlives each request, so a run
	// keeps going after POST /runs has responded.
	runCtx context.Context
	svc    Service
	log    *slog.Logger
}

// New returns the API routes.
func New(runCtx context.Context, svc Service, log *slog.Logger) http.Handler {
	h := &handler{runCtx: runCtx, svc: svc, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /runs", h.postRuns)
	mux.HandleFunc("GET /postings", h.getPostings)
	return mux
}

func (h *handler) postRuns(w http.ResponseWriter, _ *http.Request) {
	err := h.svc.StartRun(h.runCtx)
	switch {
	case errors.Is(err, service.ErrRunInProgress):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case err != nil:
		h.log.Error("start run", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not start run"})
	default:
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
	}
}

func (h *handler) getPostings(w http.ResponseWriter, r *http.Request) {
	postings, err := h.svc.ListPostings(r.Context())
	if err != nil {
		h.log.Error("list postings", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list postings"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"postings": postings})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
