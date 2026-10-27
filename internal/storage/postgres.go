package storage

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"job-alert/internal/domain"
)

// postingRow is the database shape of a posting. It is kept here so that the
// domain package does not depend on GORM.
type postingRow struct {
	Source      string    `gorm:"primaryKey"`
	Company     string    `gorm:"primaryKey"`
	JobID       string    `gorm:"primaryKey"`
	Title       string    `gorm:"not null"`
	Location    string    `gorm:"not null"`
	URL         string    `gorm:"not null"`
	FirstSeenAt time.Time `gorm:"not null"` // set on insert only
	LastSeenAt  time.Time `gorm:"not null"` // refreshed every time the posting is seen
}

func (postingRow) TableName() string { return "postings" }

func toRow(p domain.Posting, seenAt time.Time) postingRow {
	return postingRow{
		Source:      p.Source,
		Company:     p.Company,
		JobID:       p.JobID,
		Title:       p.Title,
		Location:    p.Location,
		URL:         p.URL,
		FirstSeenAt: seenAt,
		LastSeenAt:  seenAt,
	}
}

func (r postingRow) toDomain() domain.Posting {
	return domain.Posting{
		Source:   r.Source,
		Company:  r.Company,
		JobID:    r.JobID,
		Title:    r.Title,
		Location: r.Location,
		URL:      r.URL,
	}
}

// Postgres is a PostgreSQL-backed store.
type Postgres struct {
	db *gorm.DB
}

// NewPostgres connects to the database at url and creates the postings table
// if it does not exist.
func NewPostgres(url string) (*Postgres, error) {
	db, err := gorm.Open(postgres.Open(url), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := db.AutoMigrate(&postingRow{}); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return &Postgres{db: db}, nil
}

// Close closes the database connection.
func (p *Postgres) Close() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Save stores postings and returns only the ones that were not stored before.
// A posting that already exists has its title, location and URL updated and
// is not returned.
//
// It is a single INSERT ... ON CONFLICT DO UPDATE, so there is no gap between
// checking and inserting. first_seen_at is not updated on conflict, so a row
// is new exactly when the returned first_seen_at is this call's timestamp.
func (p *Postgres) Save(ctx context.Context, postings []domain.Posting) ([]domain.Posting, error) {
	// Postgres keeps microseconds; truncate so the comparison below is exact.
	seenAt := time.Now().UTC().Truncate(time.Microsecond)

	// Postgres rejects an ON CONFLICT DO UPDATE that touches the same row
	// twice, so drop duplicates within this call first.
	rows := make([]postingRow, 0, len(postings))
	seen := make(map[domain.Key]bool, len(postings))
	for _, ps := range postings {
		if seen[ps.Key()] {
			continue
		}
		seen[ps.Key()] = true
		rows = append(rows, toRow(ps, seenAt))
	}
	if len(rows) == 0 {
		return nil, nil
	}

	err := p.db.WithContext(ctx).
		Clauses(
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "source"}, {Name: "company"}, {Name: "job_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"title", "location", "url", "last_seen_at"}),
			},
			clause.Returning{}, // RETURNING *, scanned back into rows
		).
		Create(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("save postings: %w", err)
	}

	var fresh []domain.Posting
	for _, r := range rows {
		if r.FirstSeenAt.Equal(seenAt) {
			fresh = append(fresh, r.toDomain())
		}
	}
	return fresh, nil
}

// List returns all stored postings, oldest first. Postings first seen in the
// same Save are ordered by their key.
func (p *Postgres) List(ctx context.Context) ([]domain.Posting, error) {
	var rows []postingRow
	err := p.db.WithContext(ctx).
		Order("first_seen_at, source, company, job_id").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list postings: %w", err)
	}

	out := make([]domain.Posting, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toDomain())
	}
	return out, nil
}
