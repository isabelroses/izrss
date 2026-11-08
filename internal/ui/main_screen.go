package ui

import (
	"log"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/rss"
)

const (
	allPostsKey  = "\x00all"
	narrowWidth  = 70 // below this only the focused pane is drawn
	jumpDistance = 5
)

// postItemKey identifies a posts-pane row by feed and post, since one post
// can be cross-posted in several feeds.
func postItemKey(feedURL, key string) string { return feedURL + "\x00" + key }

func splitItemKey(k string) (feedURL, key string) {
	feedURL, key, _ = strings.Cut(k, "\x00")
	return feedURL, key
}

func unreadText(n int) string {
	if n == 0 {
		return "·"
	}
	return strconv.Itoa(n)
}

// rebuildFeeds refreshes the feeds pane (and so the posts pane), keeping the
// selected feed and post by identity.
func (m *Model) rebuildFeeds() {
	keep, _ := m.feedList.selected()
	total := m.feeds.GetTotalUnreads()
	items := make([]item, 0, len(m.feeds)+1)
	items = append(items, item{key: allPostsKey, title: "All posts", right: unreadText(total), strong: total > 0})
	for _, f := range m.feeds {
		n := f.GetTotalUnreads()
		items = append(items, item{key: f.URL, title: f.Title, right: unreadText(n), strong: n > 0})
	}
	m.feedList.setItems(items, keep.key)
	m.rebuildPosts()
}

// rebuildPosts lists the selected feed's posts. The post selection is kept
// while the same feed stays selected and reset when the feed changes.
func (m *Model) rebuildPosts() {
	feed, ok := m.feedList.selected()
	if !ok {
		m.postList.setItems(nil, "")
		return
	}
	keep := ""
	if feed.key == m.postsFor {
		sel, _ := m.postList.selected()
		keep = sel.key
	} else {
		m.postList.cursor, m.postList.offset = 0, 0
		m.postsFor = feed.key
	}

	type ref struct{ feed, post int }
	var refs []ref
	for i, f := range m.feeds {
		if feed.key != allPostsKey && f.URL != feed.key {
			continue
		}
		for j := range f.Posts {
			refs = append(refs, ref{i, j})
		}
	}
	all := feed.key == allPostsKey
	if all {
		slices.SortStableFunc(refs, func(a, b ref) int {
			return m.feeds[b.feed].Posts[b.post].Published.Compare(m.feeds[a.feed].Posts[a.post].Published)
		})
	}

	items := make([]item, len(refs))
	for n, r := range refs {
		f, p := m.feeds[r.feed], m.feeds[r.feed].Posts[r.post]
		right := p.Date
		if all {
			right = f.Title + " · " + p.Date
		}
		marker := " "
		if !p.Read {
			marker = "•"
		}
		items[n] = item{key: postItemKey(f.URL, p.Key()), title: p.Title, right: right, marker: marker, strong: !p.Read}
	}
	m.postList.setItems(items, keep)
}

// selectedPost locates the post under the posts-pane cursor.
func (m Model) selectedPost() (rss.Loc, bool) {
	it, ok := m.postList.selected()
	if !ok {
		return rss.Loc{}, false
	}
	return m.feeds.Locate(splitItemKey(it.key))
}

// selectedFeedIndexes are the feeds the feeds-pane selection stands for.
func (m Model) selectedFeedIndexes() []int {
	it, ok := m.feedList.selected()
	if !ok {
		return nil
	}
	var idx []int
	for i, f := range m.feeds {
		if it.key == allPostsKey || f.URL == it.key {
			idx = append(idx, i)
		}
	}
	return idx
}

func (m *Model) mainKey(msg tea.KeyPressMsg) tea.Cmd {
	k := bindings
	switch {
	case key.Matches(msg, k.Quit):
		return m.quit()
	case key.Matches(msg, k.Up):
		m.moveFocused(-1)
	case key.Matches(msg, k.Down):
		m.moveFocused(1)
	case key.Matches(msg, k.JumpUp):
		m.moveFocused(-jumpDistance)
	case key.Matches(msg, k.JumpDown):
		m.moveFocused(jumpDistance)
	case key.Matches(msg, k.Left):
		m.focus = paneFeeds
	case key.Matches(msg, k.Right):
		m.focus = panePosts
	case key.Matches(msg, k.Tab):
		m.focus = 1 - m.focus
	case key.Matches(msg, k.Open):
		if m.focus == paneFeeds {
			m.focus = panePosts
			return nil
		}
		if loc, ok := m.selectedPost(); ok {
			return m.openReader(loc)
		}
	case key.Matches(msg, k.Toggle):
		if loc, ok := m.selectedPost(); ok && m.focus == panePosts {
			return m.toggleRead(loc)
		}
	case key.Matches(msg, k.MarkFeed):
		return m.markRead(m.selectedFeedIndexes())
	case key.Matches(msg, k.Refresh):
		var urls []int
		for _, i := range m.selectedFeedIndexes() {
			if u := slices.Index(m.cfg.Urls, m.feeds[i].URL); u >= 0 {
				urls = append(urls, u)
			}
		}
		return m.refreshFeeds(urls)
	case key.Matches(msg, k.RefreshAll):
		return m.refreshFeeds(m.allIndexes())
	case key.Matches(msg, k.Browser):
		if loc, ok := m.selectedPost(); ok && m.focus == panePosts {
			return m.openLink(m.feeds[loc.Feed].Posts[loc.Post].Link)
		}
	}
	return nil
}

func (m *Model) moveFocused(delta int) {
	if m.focus == paneFeeds {
		m.feedList.move(delta)
		m.rebuildPosts()
		return
	}
	m.postList.move(delta)
}

func (m *Model) toggleRead(loc rss.Loc) tea.Cmd {
	return m.setRead(loc, !m.feeds[loc.Feed].Posts[loc.Post].Read)
}

// setRead changes and saves one post's read state.
func (m *Model) setRead(loc rss.Loc, read bool) tea.Cmd {
	m.feeds.SetRead(loc, read)
	m.rebuildFeeds()
	return m.readStatusSaved(m.feeds.WritePostTrackingAt(m.db, loc))
}

func (m *Model) markRead(feedIdx []int) tea.Cmd {
	if len(feedIdx) == 0 {
		return nil
	}
	for _, i := range feedIdx {
		rss.ReadAll(m.feeds, i)
	}
	m.rebuildFeeds()
	return m.readStatusSaved(m.feeds.WriteTracking(m.db))
}

// readStatusSaved reports a failure to save read state.
func (m *Model) readStatusSaved(err error) tea.Cmd {
	if err == nil {
		return nil
	}
	log.Printf("saving read status: %v", err)
	return m.notify(toastError, "couldn't save read status")
}

func (m *Model) openLink(link string) tea.Cmd {
	if err := m.openURL(link); err != nil {
		return m.notify(toastError, "couldn't open link: "+err.Error())
	}
	return nil
}

// paneWidths splits the width; split is false in narrow terminals.
func (m Model) paneWidths() (split bool, left, right int) {
	if m.width < narrowWidth {
		return false, m.width, m.width
	}
	longest := 0
	for _, it := range m.feedList.items {
		longest = max(longest, ansi.StringWidth(it.title))
	}
	left = min(max(longest+6, 20), m.width*30/100)
	return true, left, m.width - left - 1
}
