// Command jobalert checks company job boards and serves the postings over HTTP.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"job-alert/internal/filter"
	"job-alert/internal/handler"
	"job-alert/internal/service"
	"job-alert/internal/source"
	"job-alert/internal/storage"
)

// greenhouseBoards is the list of companies to check until config exists.
var greenhouseBoards = []string{
	"cloudflare", "stripe", "datadog", "gitlab", "twilio",
	"elastic", "mongodb", "grafanalabs", "cockroachlabs", "dropbox",
	"databricks", "pagerduty", "vercel", "figma", "discord",
	"reddit", "coinbase", "samsara", "fivetran", "planetscale",
}

// databaseURL is hardcoded until config exists.
const databaseURL = "postgres://jobalert:jobalert@localhost:5432jobalert?sslmode=disable"

// rules are the relevance rules until config exists.
var rules = filter.Rules{
	Include:   []string{"Go", "Golang", "Backend", "Platform"},
	Exclude:   []string{"Frontend", "Intern", "Marketing", "Designer"},
	Locations: []string{"Remote", "Belgrade"},
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	gh := source.NewGreenhouse()
	var boards []service.Board
	for _, company := range greenhouseBoards {
		boards = append(boards, service.Board{Source: gh, Company: company})
	}

	store, err := storage.NewPostgres(databaseURL)
	if err != nil {
		log.Error("open database", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	svc := service.New(boards, store, filter.New(rules), log)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           handler.New(ctx, svc, log),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Info("listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}
