package ui

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/isabelroses/izrss/internal/rss"
)

type feedsRefreshedMsg struct {
	feeds rss.Feeds
}

type feedRefreshedMsg struct {
	posts []rss.Post
	id    int
}

// loadCachedFeeds loads feeds from cache only (no network) for a fast first paint.
func (m Model) loadCachedFeeds() tea.Cmd {
	fetcher, urls, db := m.fetcher, m.cfg.Urls, m.db
	return func() tea.Msg {
		feeds := fetcher.GetAllContent(urls, true)
		if err := feeds.ReadTracking(db); err != nil {
			log.Printf("error reading tracking: %v", err)
		}
		return feedsRefreshedMsg{feeds: feeds}
	}
}

// refreshAll re-fetches every feed off the update loop so the UI never blocks.
func (m Model) refreshAll() tea.Cmd {
	fetcher, urls, db := m.fetcher, m.cfg.Urls, m.db
	return func() tea.Msg {
		feeds := fetcher.GetAllContent(urls, false)
		if err := feeds.ReadTracking(db); err != nil {
			log.Printf("error reading tracking: %v", err)
		}
		return feedsRefreshedMsg{feeds: feeds}
	}
}

func (m Model) refreshFeed(id int, url string) tea.Cmd {
	fetcher := m.fetcher
	return func() tea.Msg {
		return feedRefreshedMsg{id: id, posts: fetcher.GetPosts(url)}
	}
}

// reloadList re-renders the current listing view, keeping the cursor in place.
// It is a no-op for the reader and search views.
func (m *Model) reloadList() {
	if !m.ready {
		return
	}

	cursor := m.table.Cursor()
	switch m.context.curr {
	case "home":
		m.loadHome()
	case "mixed":
		m.loadMixed()
	case "content":
		if m.context.feed.ID >= 0 && m.context.feed.ID < len(m.context.feeds) {
			m.loadContent(m.context.feed.ID)
		}
	default:
		return
	}
	m.table.SetCursor(cursor)
}
