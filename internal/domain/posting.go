// Package domain holds the core types shared across layers.
package domain

// Posting is a single job opening from a job board.
// Its identity is (Source, Company, JobID).
type Posting struct {
	Source   string `json:"source"`
	Company  string `json:"company"`
	JobID    string `json:"job_id"`
	Title    string `json:"title"`
	Location string `json:"location"`
	URL      string `json:"url"`
}

// Key is a posting's identity. It is a struct rather than a joined string so
// that two different postings can never produce the same key, whatever
// characters their fields contain. It is comparable and can be a map key.
type Key struct {
	Source  string
	Company string
	JobID   string
}

// Key returns the posting's identity.
func (p Posting) Key() Key {
	return Key{Source: p.Source, Company: p.Company, JobID: p.JobID}
}
