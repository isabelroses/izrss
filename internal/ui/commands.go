package ui

import (
	"fmt"
	"log"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/isabelroses/izrss/internal/readstate"
	"github.com/isabelroses/izrss/internal/refresh"
	"github.com/isabelroses/izrss/internal/rss"
)

// feedsRefreshedMsg delivers the cached feeds at startup.
type feedsRefreshedMsg struct {
	feeds rss.Feeds
	read  readstate.Snapshot // saved read state; nil if not loaded
	at    time.Time          // last network refresh, if known
	errs  []string           // toast texts
}

// refreshMsg asks for the feeds with the given URLs to be refreshed; Init
// sends it once the cache has painted.
type refreshMsg struct{ urls []string }

// feedLoadedMsg is one feed's refresh result.
type feedLoadedMsg struct{ feed rss.Feed }

// loadCachedFeeds loads feeds from cache only, for a fast first paint; the
// refresh that follows fetches whatever is missing.
func (m Model) loadCachedFeeds() tea.Cmd {
	fetcher, urls, db := m.fetcher, m.cfg.Urls, m.db
	return func() tea.Msg {
		feeds := fetcher.GetCachedContent(urls)
		msg := feedsRefreshedMsg{feeds: feeds}
		if snap, err := readstate.Read(db); err != nil {
			log.Printf("reading read status: %v", err)
			msg.errs = append(msg.errs, "couldn't load read status")
		} else {
			msg.read = snap
		}
		if at, err := db.GetCacheTime(); err == nil && at != nil {
			msg.at = *at
		}
		return msg
	}
}

// refreshFeeds re-fetches the feeds with the given URLs, one command per
// feed so results (and progress) arrive as each finishes. What the refresh
// is already doing is left alone, so overlapping requests (r then R) merge.
func (m *Model) refreshFeeds(urls []string) tea.Cmd {
	if len(urls) == 0 || m.fetcher == nil {
		return nil
	}
	fetcher := m.fetcher
	var cmds []tea.Cmd
	for _, url := range m.refresh.Start(urls) {
		cmds = append(cmds, func() tea.Msg {
			return feedLoadedMsg{feed: fetcher.GetContentForURL(url, false)}
		})
	}
	return tea.Batch(cmds...)
}

// feedLoaded reacts to a refreshed feed: the refresh module has updated the
// feeds, so this redraws what changed and reports what went wrong.
func (m *Model) feedLoaded(msg feedLoadedMsg) tea.Cmd {
	var cmd tea.Cmd
	res := m.refresh.Finish(msg.feed, m.now())
	if res.Finished {
		db := m.db
		cmd = func() tea.Msg {
			if err := db.SetCacheTime(); err != nil {
				log.Printf("writing refresh time: %v", err)
				return notifyErr("couldn't save refresh time")
			}
			return nil
		}
	}
	if res.Err != nil {
		log.Printf("refreshing %s: %v", msg.feed.URL, res.Err)
		return tea.Batch(cmd, m.notify(toastError, res.Name+": "+shortReason(res.Err)))
	}
	if res.CacheErr != nil {
		log.Printf("%v", res.CacheErr)
		cmd = tea.Batch(cmd, m.notify(toastError, "couldn't cache "+res.Name+" feed"))
	}
	m.researchPalette()
	m.rebuildFeeds()
	if m.screen == screenReader {
		m.reopen()
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
