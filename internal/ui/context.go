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

// StartAsyncLoading enables async loading mode
func (m *Model) StartAsyncLoading() {
	m.loading = true
	m.loadedCount = 0
}
