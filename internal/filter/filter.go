// Package filter decides whether a posting is relevant.
package filter

import (
	"regexp"
	"strings"

	"job-alert/internal/domain"
)

// Rules are the relevance rules. An empty list means no constraint.
type Rules struct {
	Include   []string // title must contain at least one
	Exclude   []string // title must contain none
	Locations []string // location must contain at least one
}

// Filter applies Rules to postings.
type Filter struct {
	include   []*regexp.Regexp
	exclude   []*regexp.Regexp
	locations []*regexp.Regexp
}

// New compiles the rules once. Words match case-insensitively and only as
// whole words, so "Go" matches "Go/Rust Engineer" but not "Google".
func New(r Rules) *Filter {
	return &Filter{
		include:   compile(r.Include),
		exclude:   compile(r.Exclude),
		locations: compile(r.Locations),
	}
}

// Match reports whether p passes all rules.
func (f *Filter) Match(p domain.Posting) bool {
	if len(f.include) > 0 && !anyMatch(f.include, p.Title) {
		return false
	}
	if anyMatch(f.exclude, p.Title) {
		return false
	}
	if len(f.locations) > 0 && !anyMatch(f.locations, p.Location) {
		return false
	}
	return true
}

func compile(words []string) []*regexp.Regexp {
	var res []*regexp.Regexp
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		// A word boundary is "not a letter or digit" rather than \b, so that
		// words like "C++" work too.
		res = append(res, regexp.MustCompile(`(?i)(?:^|[^\pL\pN])`+regexp.QuoteMeta(w)+`(?:$|[^\pL\pN])`))
	}
	return res
}

func anyMatch(res []*regexp.Regexp, s string) bool {
	for _, re := range res {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
