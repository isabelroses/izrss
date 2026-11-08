package rss

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func TestSortPosts(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	posts := []Post{
		{Title: "old", Published: base},
		{Title: "new", Published: base.Add(48 * time.Hour)},
		{Title: "mid", Published: base.Add(24 * time.Hour)},
	}

	SortPosts(posts)

	want := []string{"new", "mid", "old"}
	for i, w := range want {
		if posts[i].Title != w {
			t.Errorf("position %d: expected %q, got %q", i, w, posts[i].Title)
		}
	}
}

func TestParseDate(t *testing.T) {
	parsed := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		item        *gofeed.Item
		wantDisplay string
		wantZero    bool
	}{
		{
			name:        "prefers pre-parsed timestamp",
			item:        &gofeed.Item{Published: "ignored", PublishedParsed: &parsed},
			wantDisplay: "2026-06-29",
		},
		{
			name:        "falls back to layout matching",
			item:        &gofeed.Item{Published: "02-Jan-2006"},
			wantDisplay: "2006-01-02",
		},
		{
			name:        "passes through unparseable input",
			item:        &gofeed.Item{Published: "not a date"},
			wantDisplay: "not a date",
			wantZero:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, display := parseDate(tt.item, "2006-01-02")
			if display != tt.wantDisplay {
				t.Errorf("display: expected %q, got %q", tt.wantDisplay, display)
			}
			if got.IsZero() != tt.wantZero {
				t.Errorf("zero time: expected %v, got %v (%v)", tt.wantZero, got.IsZero(), got)
			}
		})
	}
}
