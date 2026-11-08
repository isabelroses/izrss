package rss

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func TestSortedPosts_NewestFirst(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	item := func(title string, at time.Time) *gofeed.Item {
		return &gofeed.Item{Title: title, PublishedParsed: &at}
	}
	feed := &gofeed.Feed{Items: []*gofeed.Item{
		item("old", base), item("new", base.Add(48*time.Hour)), item("mid", base.Add(24*time.Hour)),
	}}

	posts := sortedPosts(feed)

	want := []string{"new", "mid", "old"}
	for i, w := range want {
		if posts[i].Title != w {
			t.Errorf("position %d: expected %q, got %q", i, w, posts[i].Title)
		}
	}
}

func TestCreatePost_Date(t *testing.T) {
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
			item:        &gofeed.Item{Published: "not a\x1b date"},
			wantDisplay: "not a date",
			wantZero:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := createPost(tt.item)
			if display := p.Date("2006-01-02"); display != tt.wantDisplay {
				t.Errorf("display: expected %q, got %q", tt.wantDisplay, display)
			}
			if p.Published.IsZero() != tt.wantZero {
				t.Errorf("zero time: expected %v, got %v (%v)", tt.wantZero, p.Published.IsZero(), p.Published)
			}
		})
	}
}
