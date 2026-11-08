package ui

import (
	"errors"
	"log"
	"strconv"

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

func unreadText(n int) string {
	if n == 0 {
		return "·"
	}
	return strconv.Itoa(n)
}

// rebuildFeeds refreshes the feeds pane (and so the posts pane), keeping the
// selected feed and post by identity. It also totals what every frame shows,
// so drawing doesn't walk every post or measure every feed title.
func (m *Model) rebuildFeeds() {
	keep, _ := m.feedList.selected()
	feeds := m.feeds.All() // positions are only used within this rebuild
	total := 0
	items := make([]item, 0, len(feeds)+1)
	items = append(items, item{key: allPostsKey, title: "All posts", right: unreadText(total), strong: total > 0})
	for _, f := range feeds {
		n := m.read.Unread(f)
		total += n
		items = append(items, item{key: f.URL, title: f.Name(), right: unreadText(n), strong: n > 0})
	}
	items[0].right, items[0].strong = unreadText(total), total > 0
	m.feedList.setItems(items, keep)
	m.unread = total
	m.feedTitleW = 0
	for _, it := range items {
		m.feedTitleW = max(m.feedTitleW, ansi.StringWidth(it.title))
	}
	m.rebuildPosts()
}

// rebuildPosts lists the selected feed's posts. The post selection is kept
// while the same feed stays selected and reset when the feed changes.
func (m *Model) rebuildPosts() {
	feed, ok := m.feedList.selected()
	if !ok {
		m.postList.setItems(nil, item{})
		return
	}
	var keep item
	if feed.key == m.postsFor {
		keep, _ = m.postList.selected()
	} else {
		m.postList.cursor, m.postList.offset = 0, 0
		m.postsFor = feed.key
	}

	feeds := m.feeds.All() // positions are only used within this rebuild
	var refs []postRef
	for i, f := range feeds {
		if feed.key != allPostsKey && f.URL != feed.key {
			continue
		}
		for j, p := range f.Posts {
			refs = append(refs, postRef{sec: p.Published.Unix(), nsec: int32(p.Published.Nanosecond()), feed: int32(i), post: int32(j)})
		}
	}
	all := feed.key == allPostsKey
	if all {
		refs = sortNewestFirst(refs)
	}

	var names []string // the feed column of "All posts"
	if all {
		names = make([]string, len(feeds))
		for i, f := range feeds {
			names[i] = f.Name()
		}
	}
	items := make([]item, len(refs))
	flags := make([][]bool, len(feeds)) // by feed, fetched as the feed's first post is met
	for n, r := range refs {
		f, p := &feeds[r.feed], &feeds[r.feed].Posts[r.post]
		if flags[r.feed] == nil {
			flags[r.feed] = m.read.Flags(*f)
		}
		source := ""
		if all {
			source = names[r.feed]
		}
		marker := " "
		unread := !flags[r.feed][r.post]
		if unread {
			marker = "•"
		}
		items[n] = item{group: f.URL, key: p.Key(), title: p.Title, source: source, right: p.Date, marker: marker, strong: unread}
	}
	m.postList.setItems(items, keep)
}

// postRef is a post's place in m.feeds and its publish time, as Unix
// seconds and nanoseconds, which order as time.Time.Compare does.
type postRef struct {
	sec              int64
	nsec, feed, post int32
}

// newer reports whether a was published after b.
func (a postRef) newer(b postRef) bool {
	return a.sec > b.sec || a.sec == b.sec && a.nsec > b.nsec
}

// sortNewestFirst sorts refs newest first, stably, returning the sorted
// slice (refs or a buffer of the same length). It is a natural merge sort:
// each feed's posts are already newest first, so "All posts" is a merge of
// a few long runs rather than a full sort.
func sortNewestFirst(refs []postRef) []postRef {
	// Split into maximal runs that are already in order.
	bounds := []int{0}
	for i := 1; i < len(refs); i++ {
		if refs[i].newer(refs[i-1]) {
			bounds = append(bounds, i)
		}
	}
	bounds = append(bounds, len(refs))

	// Merge adjacent runs pairwise until one is left; taking from the left
	// run unless the right one is strictly newer keeps the sort stable.
	src, dst := refs, make([]postRef, len(refs))
	for len(bounds) > 2 {
		merged := bounds[:1]
		for k := 0; k+1 < len(bounds); k += 2 {
			lo, mid := bounds[k], bounds[k+1]
			if k+2 >= len(bounds) { // an odd run out: copy it over
				copy(dst[lo:mid], src[lo:mid])
				merged = append(merged, mid)
				break
			}
			hi := bounds[k+2]
			i, j, o := lo, mid, lo
			for i < mid && j < hi {
				if src[j].newer(src[i]) {
					dst[o] = src[j]
					j++
				} else {
					dst[o] = src[i]
					i++
				}
				o++
			}
			o += copy(dst[o:], src[i:mid])
			copy(dst[o:], src[j:hi])
			merged = append(merged, hi)
		}
		bounds = merged
		src, dst = dst, src
	}
	return src
}

// selectedPost finds the post under the posts-pane cursor.
func (m Model) selectedPost() (rss.PostRef, rss.Post, bool) {
	it, ok := m.postList.selected()
	if !ok {
		return rss.PostRef{}, rss.Post{}, false
	}
	ref := rss.PostRef{FeedURL: it.group, Key: it.key}
	p, ok := m.feeds.Post(ref)
	return ref, p, ok
}

// selectedFeedURLs are the feeds the feeds-pane selection stands for.
func (m Model) selectedFeedURLs() []string {
	it, ok := m.feedList.selected()
	switch {
	case !ok:
		return nil
	case it.key == allPostsKey:
		return m.feeds.URLs()
	case m.feeds.Has(it.key):
		return []string{it.key}
	}
	return nil
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
		if ref, _, ok := m.selectedPost(); ok {
			return m.openReader(ref)
		}
	case key.Matches(msg, k.Toggle):
		if ref, _, ok := m.selectedPost(); ok && m.focus == panePosts {
			return m.setRead(ref, !m.read.IsRead(ref.FeedURL, ref.Key))
		}
	case key.Matches(msg, k.MarkFeed):
		return m.markRead(m.selectedFeedURLs())
	case key.Matches(msg, k.Refresh):
		return m.refreshFeeds(m.selectedFeedURLs())
	case key.Matches(msg, k.RefreshAll):
		return m.refreshFeeds(m.cfg.Urls)
	case key.Matches(msg, k.Browser):
		if _, p, ok := m.selectedPost(); ok && m.focus == panePosts {
			return m.openLink(p.Link)
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

// setRead changes and saves one post's read state.
func (m *Model) setRead(ref rss.PostRef, read bool) tea.Cmd {
	if _, ok := m.feeds.Post(ref); !ok {
		return nil
	}
	err := m.read.Set(ref, read)
	m.rebuildFeeds()
	return m.readStatusSaved(err)
}

func (m *Model) markRead(urls []string) tea.Cmd {
	if len(urls) == 0 {
		return nil
	}
	var err error
	for _, url := range urls {
		if f, ok := m.feeds.Feed(url); ok {
			err = errors.Join(err, m.read.MarkFeedRead(f))
		}
	}
	m.rebuildFeeds()
	return m.readStatusSaved(err)
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
	left = min(max(m.feedTitleW+6, 20), m.width*30/100)
	return true, left, m.width - left - 1
}
