package ui

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/library"
	"github.com/isabelroses/izrss/internal/rss"
)

const (
	allPostsKey  = "\x00all"
	narrowWidth  = 70 // below this only the focused pane is drawn
	jumpDistance = 5
)

// mainPage is the main screen: the feeds pane, the posts pane showing the
// feed selected in it, which of the two has focus, and their layout. It
// follows the library on its own (Sync), keeping selections by identity;
// Model passes it keys and sizes and asks what is selected.
type mainPage struct {
	lib        *library.Library
	home       string // "mixed" starts on all posts
	dateFormat string

	width, height int
	focus         pane
	feeds, posts  list
	postsFor      string // the feed key the posts pane was built for
	builtAt       int    // the library version the panes show; -1 before the first build
	started       bool   // the first feed selection is made
	unread        int    // unread posts in all feeds, as of the last build
	titleW        int    // the widest feeds-pane title, as of the last build
}

func newMainPage(lib *library.Library, home, dateFormat string) mainPage {
	p := mainPage{lib: lib, home: home, dateFormat: dateFormat, builtAt: -1}
	p.Sync()
	return p
}

// Sync rebuilds the panes if the library changed since they were built.
func (p *mainPage) Sync() {
	if p.lib.Version() == p.builtAt {
		return
	}
	p.builtAt = p.lib.Version()
	p.rebuildFeeds()
	if !p.started && p.lib.Len() > 0 { // the first feeds have arrived
		p.started = true
		if p.home != "mixed" && len(p.feeds.items) > 1 {
			p.selectFeed(1)
		}
	}
}

// Resize lays the page out for a width×height terminal.
func (p *mainPage) Resize(width, height int) {
	p.width, p.height = width, height
	body := max(height-1, 0) // the hint/status line
	p.feeds.setHeight(body)
	p.posts.setHeight(body)
}

// Key moves the selection or the focus, reporting whether msg was one of
// those keys.
func (p *mainPage) Key(msg tea.KeyPressMsg) bool {
	k := bindings
	switch {
	case key.Matches(msg, k.Up):
		p.move(-1)
	case key.Matches(msg, k.Down):
		p.move(1)
	case key.Matches(msg, k.JumpUp):
		p.move(-jumpDistance)
	case key.Matches(msg, k.JumpDown):
		p.move(jumpDistance)
	case key.Matches(msg, k.Left):
		p.focus = paneFeeds
	case key.Matches(msg, k.Right), key.Matches(msg, k.Open) && p.focus == paneFeeds:
		p.focus = panePosts
	case key.Matches(msg, k.Tab):
		p.focus = 1 - p.focus
	default:
		return false
	}
	return true
}

// Focus is the pane with focus.
func (p *mainPage) Focus() pane { return p.focus }

// Unread is how many posts in all feeds are unread.
func (p *mainPage) Unread() int { return p.unread }

// SelectedPost is the post under the posts-pane cursor, while that pane has
// focus.
func (p *mainPage) SelectedPost() (library.Entry, bool) {
	it, ok := p.posts.selected()
	if !ok || p.focus != panePosts {
		return library.Entry{}, false
	}
	return p.lib.Entry(postRef(it))
}

// SelectedFeeds are the feeds the feeds-pane selection stands for.
func (p *mainPage) SelectedFeeds() []string {
	it, ok := p.feeds.selected()
	switch {
	case !ok:
		return nil
	case it.key == allPostsKey:
		return p.lib.URLs()
	}
	if _, ok := p.lib.Feed(it.key); ok {
		return []string{it.key}
	}
	return nil
}

// postRef is the post a posts-pane row shows.
func postRef(it item) rss.PostRef { return rss.PostRef{FeedURL: it.group, Key: it.key} }

func (p *mainPage) move(delta int) {
	if p.focus == paneFeeds {
		p.selectFeed(p.feeds.cursor + delta)
		return
	}
	p.posts.move(delta)
}

// selectFeed puts the feeds-pane cursor on row i and shows that feed's
// posts: the posts pane always follows the feed cursor.
func (p *mainPage) selectFeed(i int) {
	p.feeds.move(i - p.feeds.cursor)
	p.rebuildPosts()
}

func unreadText(n int) string {
	if n == 0 {
		return "·"
	}
	return strconv.Itoa(n)
}

// rebuildFeeds lists the feeds, keeping the selected one by identity, then
// the posts pane. It also totals what every frame shows, so drawing doesn't
// walk every post or measure every feed title.
func (p *mainPage) rebuildFeeds() {
	keep, _ := p.feeds.selected()
	feeds := p.lib.Feeds()
	total := 0
	items := make([]item, 0, len(feeds)+1)
	items = append(items, item{key: allPostsKey, title: "All posts"})
	for _, f := range feeds {
		total += f.Unread
		items = append(items, item{key: f.URL, title: f.Name, right: unreadText(f.Unread), strong: f.Unread > 0})
	}
	items[0].right, items[0].strong = unreadText(total), total > 0
	p.feeds.setItems(items, keep)
	p.unread = total
	p.titleW = 0
	for _, it := range items {
		p.titleW = max(p.titleW, ansi.StringWidth(it.title))
	}
	p.rebuildPosts()
}

// rebuildPosts lists the selected feed's posts. The post selection is kept
// while the same feed stays selected and reset when the feed changes.
func (p *mainPage) rebuildPosts() {
	feed, ok := p.feeds.selected()
	if !ok {
		p.posts.setItems(nil, item{})
		return
	}
	var keep item
	if feed.key == p.postsFor {
		keep, _ = p.posts.selected()
	} else {
		p.posts.reset()
		p.postsFor = feed.key
	}

	all := feed.key == allPostsKey
	var entries []library.Entry
	if all {
		entries = p.lib.AllPosts()
	} else {
		entries = p.lib.Posts(feed.key)
	}
	items := make([]item, len(entries))
	for i, e := range entries {
		source := ""
		if all {
			source = e.Feed
		}
		marker := " "
		if !e.Read {
			marker = "•"
		}
		items[i] = item{group: e.Ref.FeedURL, key: e.Ref.Key, title: e.Post.Title, source: source,
			right: e.Post.Date(p.dateFormat), marker: marker, strong: !e.Read}
	}
	p.posts.setItems(items, keep)
}

// paneWidths splits the width; split is false in narrow terminals.
func (p *mainPage) paneWidths() (split bool, left, right int) {
	if p.width < narrowWidth {
		return false, p.width, p.width
	}
	left = min(max(p.titleW+6, 20), p.width*30/100)
	return true, left, p.width - left - 1
}

// View draws the screen: the panes, then status, the hint line.
func (p *mainPage) View(st *Styles, status string) string {
	body := max(p.height-1, 0)
	lines := make([]string, 0, p.height)
	switch split, left, right := p.paneWidths(); {
	case p.lib.Len() == 0:
		lines = append(lines, placeholderLines("No feeds loaded yet", p.width, body, st.Hint)...)
	case split:
		l := p.feeds.view(left, p.focus == paneFeeds, st.List)
		r := p.postsPane(right, st)
		sep := st.Border.Render("│")
		for i := range body {
			lines = append(lines, l[i]+sep+r[i])
		}
	case p.focus == paneFeeds:
		lines = append(lines, p.feeds.view(p.width, true, st.List)...)
	default:
		lines = append(lines, p.postsPane(p.width, st)...)
	}
	return strings.Join(append(lines, status), "\n")
}

func (p *mainPage) postsPane(width int, st *Styles) []string {
	if len(p.posts.items) == 0 {
		return placeholderLines("No posts", width, p.posts.height, st.Hint)
	}
	return p.posts.view(width, p.focus == panePosts, st.List)
}
