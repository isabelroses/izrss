package library

import (
	"slices"
	"testing"
	"time"

	"github.com/isabelroses/izrss/internal/rss"
)

// Matching ignores case; title matches at a word start rank above ones
// mid-word, above content-only ones; within a rank, newer posts come first.
func TestSearchPosts(t *testing.T) {
	t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	at := func(h int) time.Time { return t0.Add(-time.Duration(h) * time.Hour) }
	feeds := []rss.Feed{
		{URL: "a", Posts: []rss.Post{
			{Title: "Weekly links", Content: "<p>the release went out</p>", Published: at(0)},
			{Title: "Prerelease builds", Content: "x", Published: at(1)},
			{Title: "Release notes 1.2", Content: "x", Published: at(5)},
		}},
		{URL: "b", Posts: []rss.Post{
			{Title: "On shipping", Content: "<p>RELEASE early</p>", Published: at(2)},
			{Title: "Release notes 1.3", Content: "x", Published: at(3)},
			{Title: "Unreleased ideas", Content: "x", Published: at(4)},
			{Title: "Kelvin", Content: "Ünïcödé — “QUOTES”", Published: at(6)},
		}},
	}
	titles := func(query string) []string {
		var got []string
		for _, ref := range searchPosts(feeds, query) {
			p, _ := rss.NewFeedSet(feeds...).Post(ref)
			got = append(got, p.Title)
		}
		return got
	}
	for query, want := range map[string][]string{
		"RELEASE": {"Release notes 1.3", "Release notes 1.2", "Prerelease builds", "Unreleased ideas", "Weekly links", "On shipping"},
		"kelvin":  {"Kelvin"},
		"quotes":  {"Kelvin"},
		"python":  nil,
		" ":       nil,
		"":        nil,
	} {
		if got := titles(query); !slices.Equal(got, want) {
			t.Errorf("searchPosts(%q) = %q, want %q", query, got, want)
		}
	}
}
