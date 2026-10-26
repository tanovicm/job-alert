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

// Key returns the posting's identity as a single string.
func (p Posting) Key() string {
	return p.Source + "/" + p.Company + "/" + p.JobID
}
