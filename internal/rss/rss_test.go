package rss

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func TestGetTotalUnreads(t *testing.T) {
	feed := Feed{
		Posts: []Post{
			{UUID: "1", Read: false},
			{UUID: "2", Read: true},
			{UUID: "3", Read: false},
		},
	}

	total := feed.GetTotalUnreads()
	if total != 2 {
		t.Errorf("Expected 2 unread posts, got %d", total)
	}
}

func TestFeedsGetTotalUnreads(t *testing.T) {
	feeds := Feeds{
		{Posts: []Post{
			{UUID: "1", Read: false},
			{UUID: "2", Read: true},
		}},
		{Posts: []Post{
			{UUID: "3", Read: false},
			{UUID: "4", Read: false},
		}},
	}

	total := feeds.GetTotalUnreads()
	if total != 3 {
		t.Errorf("Expected 3 unread posts, got %d", total)
	}
}

func TestToggleRead(t *testing.T) {
	feeds := Feeds{
		{Posts: []Post{
			{UUID: "1", Read: false},
			{UUID: "2", Read: true},
		}},
	}

	// Toggle first post (false -> true)
	ToggleRead(feeds, 0, 0)
	if !feeds[0].Posts[0].Read {
		t.Errorf("Expected post to be read after toggle")
	}

	// Toggle again (true -> false)
	ToggleRead(feeds, 0, 0)
	if feeds[0].Posts[0].Read {
		t.Errorf("Expected post to be unread after second toggle")
	}
}

func TestReadAll(t *testing.T) {
	feeds := Feeds{
		{Posts: []Post{
			{UUID: "1", Read: false},
			{UUID: "2", Read: false},
		}},
	}

	ReadAll(feeds, 0)

	for _, post := range feeds[0].Posts {
		if !post.Read {
			t.Errorf("Expected all posts to be read")
		}
	}
}

func TestMarkRead(t *testing.T) {
	feeds := Feeds{
		{Posts: []Post{
			{UUID: "1", Read: false},
		}},
	}

	MarkRead(feeds, 0, 0)

	if !feeds[0].Posts[0].Read {
		t.Errorf("Expected post to be marked as read")
	}
}

func TestReadSymbol(t *testing.T) {
	if ReadSymbol(true) != "" {
		t.Errorf("Expected empty string for read post")
	}
	if ReadSymbol(false) != "•" {
		t.Errorf("Expected bullet for unread post")
	}
}

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
			item:        &gofeed.Item{Published: "Mon, 02 Jan 2006 15:04:05 -0700"},
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
