package cmd

import (
	"github.com/isabelroses/izrss/lib"
)

type context struct {
	prev  string
	curr  string
	feeds lib.Feeds
	post  lib.Post
	feed  lib.Feed
}

func (m *Model) swapPage(next string) {
	m.context.prev = m.context.curr
	m.context.curr = next
	if m.context.prev == "reader" {
		m.viewport.Height = m.viewport.Height + 2
	}
}
