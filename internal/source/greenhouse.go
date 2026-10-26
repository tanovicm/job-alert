package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"job-alert/internal/domain"
)

// Greenhouse reads the public Greenhouse Job Board API.
// https://developers.greenhouse.io/job-board.html
type Greenhouse struct {
	BaseURL string
	Client  *http.Client
}

func NewGreenhouse() *Greenhouse {
	return &Greenhouse{
		BaseURL: "https://boards-api.greenhouse.io",
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (g *Greenhouse) Name() string { return "greenhouse" }

type greenhouseResponse struct {
	Jobs []struct {
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		AbsoluteURL string `json:"absolute_url"`
		Location    struct {
			Name string `json:"name"`
		} `json:"location"`
	} `json:"jobs"`
}

// Fetch returns all open postings on the given Greenhouse board.
func (g *Greenhouse) Fetch(ctx context.Context, board string) ([]domain.Posting, error) {
	u := g.BaseURL + "/v1/boards/" + url.PathEscape(board) + "/jobs"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: unexpected status %s", u, resp.Status)
	}

	var body greenhouseResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("GET %s: decode response: %w", u, err)
	}

	postings := make([]domain.Posting, 0, len(body.Jobs))
	for _, j := range body.Jobs {
		postings = append(postings, domain.Posting{
			Source:   g.Name(),
			Company:  board,
			JobID:    strconv.FormatInt(j.ID, 10),
			Title:    j.Title,
			Location: j.Location.Name,
			URL:      j.AbsoluteURL,
		})
	}
	return postings, nil
}
