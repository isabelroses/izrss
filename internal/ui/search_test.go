package ui

import (
	"strings"
	"testing"

	"github.com/isabelroses/izrss/internal/rss"
)

func TestAppendLower_MatchesStringsToLower(t *testing.T) {
	inputs := []string{
		"",
		"plain ascii",
		"MiXeD CaSe 123",
		"Ünïcödé — “QUOTES”",
		"İSTANBUL",          // İ lowers to a 2-byte i̇ form
		"K Kelvin",          // Kelvin sign lowers to ASCII k
		"ΣΊΣΥΦΟΣ",           // Greek capitals
		"bad \xff\xfe utf8", // invalid bytes become U+FFFD
	}
	for _, s := range inputs {
		t.Run(s, func(t *testing.T) {
			if got, want := string(appendLower(nil, s)), strings.ToLower(s); got != want {
				t.Errorf("appendLower(%q) = %q, want %q", s, got, want)
			}
		})
	}
}

func TestSearchPosts(t *testing.T) {
	feeds := rss.Feeds{
		{Posts: []rss.Post{
			{Title: "a", Content: "Go is FUN"},
			{Title: "b", Content: "Rust"},
			{Title: "Weekly digest", Content: "news"},
		}},
		{Posts: []rss.Post{
			{Title: "c", Content: "Kelvin scale"},
		}},
	}
	tests := []struct {
		query string
		want  []string
	}{
		{"fun", []string{"a"}},
		{"FUN", []string{"a"}},
		{"kelvin", []string{"c"}},
		{"", nil},
		{"weekly", []string{"Weekly digest"}},
		{"python", nil},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			var got []string
			for _, h := range newSearchIndex(feeds).search(tt.query) {
				got = append(got, feeds[h.Feed].Posts[h.Post].Title)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("search(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}
