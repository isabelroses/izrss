package rss

import (
	"testing"
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

func TestURLToDir(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://isabelroses.com/feed.xml", "isabelroses_com_feed.xml"},
		{"http://example.com/rss", "example.com_rss"},
		{"https://blog.example.org/feed.atom", "blog_example_org_feed.atom"},
	}

	for _, tt := range tests {
		result := urlToDir(tt.input)
		if result != tt.expected {
			t.Errorf("urlToDir(%q) = %q, want %q", tt.input, result, tt.expected)
		}
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
