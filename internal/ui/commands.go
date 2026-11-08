package ui

import (
	"fmt"
	"log"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/isabelroses/izrss/internal/library"
	"github.com/isabelroses/izrss/internal/refresh"
	"github.com/isabelroses/izrss/internal/rss"
)

// cacheLoadedMsg delivers the cached feeds and saved read marks at startup.
type cacheLoadedMsg struct{ snap library.Snapshot }

// refreshMsg asks for the feeds with the given URLs to be refreshed; Init
// sends it once the cache has painted.
type refreshMsg struct{ urls []string }

// feedLoadedMsg is one feed's refresh result.
type feedLoadedMsg struct{ feed rss.Feed }

// loadCachedFeeds loads feeds from cache only, for a fast first paint; the
// refresh that follows fetches whatever is missing.
func (m Model) loadCachedFeeds() tea.Cmd {
	lib, urls := m.lib, m.cfg.Urls
	return func() tea.Msg { return cacheLoadedMsg{snap: lib.Load(urls)} }
}

// refreshFeeds re-fetches the feeds with the given URLs, one command per
// feed so results (and progress) arrive as each finishes. What the refresh
// is already doing is left alone, so overlapping requests (r then R) merge.
func (m *Model) refreshFeeds(urls []string) tea.Cmd {
	var cmds []tea.Cmd
	for _, job := range m.refresh.Start(urls) {
		cmds = append(cmds, func() tea.Msg { return feedLoadedMsg{feed: job()} })
	}
	return tea.Batch(cmds...)
}

// feedLoaded reacts to a refreshed feed: the refresh module has updated the
// library, so this re-shows the open post and reports what went wrong.
func (m *Model) feedLoaded(msg feedLoadedMsg) tea.Cmd {
	var cmd tea.Cmd
	res := m.refresh.Finish(msg.feed, m.now())
	if res.Err != nil {
		log.Printf("refreshing %s: %v", msg.feed.URL, res.Err)
		return m.notify(res.Name + ": " + shortReason(res.Err))
	}
	if res.CacheErr != nil {
		log.Printf("%v", res.CacheErr)
		cmd = m.notify("couldn't cache " + res.Name + " feed")
	}
	if e, ok := m.lib.Entry(m.page.Ref()); ok && m.page.Shown() { // the feeds changed underneath the reader
		m.page.Refresh(e.Post)
	}
	return cmd
}

type minuteTickMsg struct{}

// minuteTick re-renders the "refreshed Nm ago" text once a minute.
func minuteTick() tea.Cmd {
	return tea.Tick(time.Minute, func(time.Time) tea.Msg { return minuteTickMsg{} })
}

func statusText(unread int, r *refresh.Refresher, now time.Time) string {
	if done, total := r.Progress(); total > 0 {
		return fmt.Sprintf("refreshing %d/%d…", done, total)
	}
	s := fmt.Sprintf("%d unread", unread)
	if at := r.At(); !at.IsZero() {
		s += " · refreshed " + ago(now.Sub(at))
	}
	return s
}

func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	}
	return fmt.Sprintf("%dd ago", int(d/(24*time.Hour)))
}
