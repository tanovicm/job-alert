package filter

import (
	"testing"

	"job-alert/internal/domain"
)

func TestMatch(t *testing.T) {
	f := New(Rules{
		Include:   []string{"Go", "Golang", "Backend", "Platform"},
		Exclude:   []string{"Frontend", "Intern"},
		Locations: []string{"Remote", "Belgrade"},
	})

	tests := []struct {
		title    string
		location string
		want     bool
	}{
		{"Senior Go Engineer", "Remote", true},
		{"Go/Rust Engineer", "Belgrade, Serbia", true},
		{"golang developer", "REMOTE - EU", true},
		{"Backend Engineer", "Remote", true},
		{"Google Ads Specialist", "Remote", false}, // "Go" is not a whole word here
		{"Category Manager", "Remote", false},      // neither here
		{"Frontend Platform Engineer", "Remote", false},
		{"Backend Intern", "Remote", false},
		{"Internal Tools Backend Engineer", "Remote", true}, // "Intern" is not a whole word
		{"Account Executive", "Remote", false},
		{"Senior Go Engineer", "Berlin", false},
		{"Senior Go Engineer", "Remoteville", false},
		{"Senior Go Engineer", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.title+" @ "+tt.location, func(t *testing.T) {
			got := f.Match(domain.Posting{Title: tt.title, Location: tt.location})
			if got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmptyListsMeanNoConstraint(t *testing.T) {
	tests := []struct {
		name  string
		rules Rules
		p     domain.Posting
		want  bool
	}{
		{"no rules", Rules{}, domain.Posting{Title: "Anything"}, true},
		{"no include", Rules{Exclude: []string{"Intern"}}, domain.Posting{Title: "Designer"}, true},
		{"no locations", Rules{Include: []string{"Go"}}, domain.Posting{Title: "Go Engineer"}, true},
		{"blank words ignored", Rules{Include: []string{" ", ""}}, domain.Posting{Title: "Designer"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.rules).Match(tt.p); got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}
