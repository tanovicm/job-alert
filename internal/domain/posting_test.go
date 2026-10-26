package domain

import "testing"

func TestKeyDiffersForDifferentPostings(t *testing.T) {
	tests := []struct {
		name string
		a, b Posting
	}{
		{
			name: "slash moves between company and job ID",
			a:    Posting{Source: "greenhouse", Company: "acme/eu", JobID: "1"},
			b:    Posting{Source: "greenhouse", Company: "acme", JobID: "eu/1"},
		},
		{
			name: "slash moves between source and company",
			a:    Posting{Source: "lever/eu", Company: "acme", JobID: "1"},
			b:    Posting{Source: "lever", Company: "eu/acme", JobID: "1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.a.Key() == tt.b.Key() {
				t.Errorf("different postings share key %q:\n  a = %+v\n  b = %+v", tt.a.Key(), tt.a, tt.b)
			}
		})
	}
}
