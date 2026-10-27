// Package scheduler triggers runs at a fixed interval.
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"job-alert/internal/service"
)

// Starter starts a run in the background.
type Starter interface {
	StartRun(ctx context.Context) error
}

// Run starts a run right away, then one every interval, until ctx is done.
// A tick that comes while a run is still in progress is skipped and logged.
func Run(ctx context.Context, s Starter, interval time.Duration, log *slog.Logger) {
	trigger := func() {
		err := s.StartRun(ctx)
		switch {
		case errors.Is(err, service.ErrRunInProgress):
			log.Warn("scheduled run skipped: a run is already in progress")
		case err != nil:
			log.Error("scheduled run failed to start", "err", err)
		}
	}

	log.Info("scheduler started", "interval", interval.String())
	trigger()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("scheduler stopped")
			return
		case <-ticker.C:
			trigger()
		}
	}
}
