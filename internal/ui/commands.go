package ui

import (
	"fmt"
	"log"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/isabelroses/izrss/internal/rss"
)

// feedsRefreshedMsg delivers the cached feeds at startup.
type feedsRefreshedMsg struct {
	feeds rss.Feeds
	at    time.Time // last network refresh, if known
	errs  []string  // toast texts
}

// refreshMsg asks for the feeds at the given cfg.Urls indexes to be
// refreshed; Init sends it once the cache has painted.
type refreshMsg struct{ indexes []int }

// feedLoadedMsg is one feed's refresh result; index is its position in
// cfg.Urls (and so in m.feeds).
type feedLoadedMsg struct {
	feed  rss.Feed
	index int
}

// loadCachedFeeds loads feeds from cache only, for a fast first paint; the
// refresh that follows fetches whatever is missing.
func (m Model) loadCachedFeeds() tea.Cmd {
	fetcher, urls, db := m.fetcher, m.cfg.Urls, m.db
	return func() tea.Msg {
		feeds := fetcher.GetCachedContent(urls)
		msg := feedsRefreshedMsg{feeds: feeds}
		if err := feeds.ReadTracking(db); err != nil {
			log.Printf("reading read status: %v", err)
			msg.errs = append(msg.errs, "couldn't load read status")
		}
		if at, err := db.GetCacheTime(); err == nil && at != nil {
			msg.at = *at
		}
		return msg
	}
}

func (m Model) allIndexes() []int {
	idx := make([]int, len(m.cfg.Urls))
	for i := range idx {
		idx[i] = i
	}
	return idx
}

// refreshFeeds re-fetches the feeds at the given cfg.Urls indexes, one
// command per feed so results (and progress) arrive as each finishes. Feeds
// already being refreshed are skipped, so overlapping requests (r then R)
// merge into one refresh rather than stacking.
func (m *Model) refreshFeeds(indexes []int) tea.Cmd {
	if len(indexes) == 0 || m.fetcher == nil {
		return nil
	}
	fetcher, urls := m.fetcher, m.cfg.Urls
	var cmds []tea.Cmd
	for _, i := range indexes {
		if i < 0 || i >= len(urls) || m.refresh.inFlight[i] {
			continue
		}
		if m.refresh.inFlight == nil {
			m.refresh.inFlight = map[int]bool{}
		}
		m.refresh.inFlight[i] = true
		url := urls[i]
		cmds = append(cmds, func() tea.Msg {
			return feedLoadedMsg{feed: fetcher.GetContentForURL(url, false), index: i}
		})
	}
	m.refresh.total += len(cmds)
	return tea.Batch(cmds...)
}

// feedLoaded swaps in a refreshed feed, keeping selections by identity.
func (m *Model) feedLoaded(msg feedLoadedMsg) tea.Cmd {
	var cmd tea.Cmd
	if m.refresh.inFlight[msg.index] { // a result not asked for isn't progress
		delete(m.refresh.inFlight, msg.index)
		m.refresh.done++
	}
	if m.refresh.total > 0 && m.refresh.done >= m.refresh.total {
		m.refresh = refreshState{at: m.now()}
		db := m.db
		cmd = func() tea.Msg {
			if err := db.SetCacheTime(); err != nil {
				log.Printf("writing refresh time: %v", err)
				return notifyErr("couldn't save refresh time")
			}
			return nil
		}
	}
	name := feedName(msg.feed)
	if msg.index < len(m.feeds) {
		name = feedName(m.feeds[msg.index])
	}
	if msg.feed.Err != nil {
		log.Printf("refreshing %s: %v", msg.feed.URL, msg.feed.Err)
		return tea.Batch(cmd, m.notify(toastError, name+": "+shortReason(msg.feed.Err)))
	}
	if msg.feed.CacheErr != nil {
		log.Printf("%v", msg.feed.CacheErr)
		cmd = tea.Batch(cmd, m.notify(toastError, "couldn't cache "+name+" feed"))
	}
	switch {
	case msg.index < len(m.feeds):
		m.feeds[msg.index] = msg.feed
	case msg.index == len(m.feeds):
		m.feeds = append(m.feeds, msg.feed)
	default:
		return cmd
	}
	m.feedChanged(msg.index)
	if err := m.feeds.ReadTrackingAt(m.db, msg.index); err != nil {
		log.Printf("reading read status: %v", err)
		cmd = tea.Batch(cmd, m.notify(toastError, "couldn't load read status"))
	}
	m.rebuildFeeds()
	if m.screen == screenReader {
		m.reopen()
	}
	return cmd
}

// refreshState tracks a network refresh across its per-feed results.
type refreshState struct {
	total, done int
	inFlight    map[int]bool // cfg.Urls indexes being fetched
	at          time.Time    // when the last full refresh finished
}

type minuteTickMsg struct{}

// minuteTick re-renders the "refreshed Nm ago" text once a minute.
func minuteTick() tea.Cmd {
	return tea.Tick(time.Minute, func(time.Time) tea.Msg { return minuteTickMsg{} })
}

func statusText(unread int, r refreshState, now time.Time) string {
	if r.total > 0 {
		return fmt.Sprintf("refreshing %d/%d…", r.done, r.total)
	}
	s := fmt.Sprintf("%d unread", unread)
	if !r.at.IsZero() {
		s += " · refreshed " + ago(now.Sub(r.at))
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
