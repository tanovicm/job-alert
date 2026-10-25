# Job Alert Service

A small service that regularly checks the public job boards of a list of companies and surfaces new, relevant openings.

## Requirements

- Checks job boards on a schedule (daily by default, interval configurable)
- Manual trigger via HTTP
- Lists relevant postings via HTTP
- Sources: Greenhouse and Lever public APIs (20–30 companies)
- Adding a new source should be easy
- Relevance rules come from config:
  - title contains one of the include keywords (e.g. "Go", "Golang", "Backend", "Platform")
  - title contains none of the exclude keywords (e.g. "Frontend", "Intern")
  - location matches a list (e.g. "Remote", "Belgrade"), or no location filter
- A posting is new if it hasn't been seen before; identity = source + company + job ID
  - edits to an existing posting are not new
  - a republished posting with a new ID counts as new
- History kept for ~90 days, survives restarts
- One failing company or source doesn't stop the others; it is logged and retried on the next run
- No duplicates stored or reported

## API

| Method | Path        | Description                                  |
|--------|-------------|----------------------------------------------|
| POST   | `/runs`     | Trigger a check. `202 Accepted`, or `409 Conflict` if a run is already in progress |
| GET    | `/postings` | List relevant postings                       |

## Decisions

- **Go** service with an in-process scheduler
- **PostgreSQL** for postings, unique key on (source, company, job ID)
- **Overlapping runs:** only one run at a time; a later trigger is rejected
- **Sources** implement a common `Source` interface, so new boards can be added without touching the core logic

## Out of scope / Future

- Notifications (Slack, Telegram, email)
- Per-user filters (currently one shared config for the team)
- Run status endpoint: `POST /runs` returns a run ID, `GET /runs/{id}` shows `running` / `succeeded` / `failed`
- Deployment (Docker, Kubernetes, Helm, Terraform)
- UI
