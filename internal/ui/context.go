package ui

import (
	"github.com/isabelroses/izrss/internal/rss"
)

type context struct {
	prev  string
	curr  string
	feeds rss.Feeds
	post  rss.Post
	feed  rss.Feed
}

func (m *Model) swapPage(next string) {
	m.context.prev = m.context.curr
	m.context.curr = next
	if m.context.prev == "reader" {
		m.viewport.Height = m.viewport.Height + 2
	}
}

// SetFeeds sets the feeds for the model
func (m *Model) SetFeeds(feeds rss.Feeds) {
	m.context.feeds = feeds
}
