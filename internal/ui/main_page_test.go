package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/library"
	"github.com/isabelroses/izrss/internal/refresh"
	"github.com/isabelroses/izrss/internal/rss"
)

func TestStartsOnFirstFeed(t *testing.T) {
	m := newTestModel(t)
	if m.page.Shown() || m.main.focus != paneFeeds || selectedTitle(m.main.feeds) != "Alpha" {
		t.Errorf("start: reader %v focus %v feed %q", m.page.Shown(), m.main.focus, selectedTitle(m.main.feeds))
	}
	if selectedTitle(m.main.posts) != "Alpha post 0" || len(m.main.posts.items) != 4 {
		t.Errorf("posts pane should show Alpha's posts, got %q (%d)", selectedTitle(m.main.posts), len(m.main.posts.items))
	}
}

func TestAllPostsFirstAndMixed(t *testing.T) {
	m, _ := keys(newTestModel(t), "k")
	if m.main.feeds.items[0].title != "All posts" || selectedTitle(m.main.feeds) != "All posts" {
		t.Fatalf("row 0 should be All posts, got %q", m.main.feeds.items[0].title)
	}
	if n := len(m.main.posts.items); n != 12 {
		t.Fatalf("All posts should list 12, got %d", n)
	}
	want := []string{"Alpha post 0", "Beta post 0", "Gamma post 0", "Alpha post 1"}
	for i, w := range want {
		if m.main.posts.items[i].title != w {
			t.Errorf("row %d = %q, want %q (newest first)", i, m.main.posts.items[i].title, w)
		}
	}
	if r := m.main.posts.items[1].rightText(); r != "Beta · 04/11/2026" {
		t.Errorf("All posts rows should name the feed: %q", r)
	}
	if m.main.feeds.items[0].right != "3" {
		t.Errorf("All posts unread count = %q, want 3", m.main.feeds.items[0].right)
	}
}

func TestHomeMixedSelectsAllPosts(t *testing.T) {
	cfg := config.Default()
	cfg.Home = "mixed"
	m := *NewModel(cfg, newLib(testDB(t)), nil, nil)
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = update(m, loaded(m, sampleFeeds()...))
	if selectedTitle(m.main.feeds) != "All posts" {
		t.Errorf("home = mixed should start on All posts, got %q", selectedTitle(m.main.feeds))
	}
}

func TestPaneSwitch(t *testing.T) {
	m := newTestModel(t)
	for _, k := range []string{"l", "right", "tab"} {
		m.main.focus = paneFeeds
		if m, _ = keys(m, k); m.main.focus != panePosts {
			t.Errorf("%s should focus posts", k)
		}
	}
	for _, k := range []string{"h", "left"} {
		m.main.focus = panePosts
		if m, _ = keys(m, k); m.main.focus != paneFeeds {
			t.Errorf("%s should focus feeds", k)
		}
	}
	m.main.focus = paneFeeds
	if m, _ = keys(m, "enter"); m.main.focus != panePosts || m.page.Shown() {
		t.Error("enter on feeds should focus posts, not open the reader")
	}
}

func TestToggleReadFromAllPosts(t *testing.T) {
	m, _ := keys(newTestModel(t), "k", "l", "j") // All posts, row 1 = Beta post 0 (unread)
	if selectedTitle(m.main.posts) != "Beta post 0" {
		t.Fatalf("selected %q", selectedTitle(m.main.posts))
	}
	m, _ = keys(m, "x")
	if !isRead(m, 1, 0) {
		t.Error("Beta post 0 should be read")
	}
	for i := range m.lib.Len() {
		for j, p := range feedAt(m, i).Posts {
			if i != 1 && j == 0 && isRead(m, i, j) {
				t.Errorf("%s was changed too", p.Title)
			}
		}
	}
}

func TestMarkFeedRead(t *testing.T) {
	m, _ := keys(newTestModel(t), "X")
	if m.main.unread != 2 || m.lib.Feeds()[0].Unread != 0 {
		t.Errorf("X should mark only Alpha read; unread = %d", m.main.unread)
	}
	m, _ = keys(m, "k", "X")
	if m.main.unread != 0 {
		t.Errorf("X on All posts should mark everything read")
	}
}

func TestKeys_JumpRefreshOpen(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "J")
	if m.main.posts.cursor != 3 { // 4 posts: jump 5 clamps at 3
		t.Errorf("J should jump, cursor %d", m.main.posts.cursor)
	}
	m, _ = keys(m, "K")
	if m.main.posts.cursor != 0 {
		t.Errorf("K should jump back, cursor %d", m.main.posts.cursor)
	}

	var opened string
	m.openURL = func(u string) error { opened = u; return nil }
	m, _ = keys(m, "o")
	if opened != "https://Alpha.example/0" {
		t.Errorf("o opened %q", opened)
	}
	if r, _ := update(m, press("r")); totalOf(r) != 1 {
		t.Errorf("r on a feed should refresh 1 feed, got %d", totalOf(r))
	}
	all := newTestModel(t)
	if r, _ := update(all, press("R")); totalOf(r) != 3 {
		t.Errorf("R should refresh all 3, got %d", totalOf(r))
	}
}

func TestRefresh_KeepsSelectionByKey(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "j", "j") // Alpha post 2
	f := sampleFeeds()[0]
	f.Posts = append([]rss.Post{{UUID: "new1", Title: "new 1", Published: t0.Add(time.Hour)}, {UUID: "new2", Title: "new 2", Published: t0.Add(time.Hour)}}, f.Posts...)
	m.refresh.Start([]string{sampleURL(0)})
	m, _ = update(m, feedLoadedMsg{feed: f})
	if selectedTitle(m.main.posts) != "Alpha post 2" {
		t.Errorf("selection moved to %q", selectedTitle(m.main.posts))
	}
}

func TestRefresh_SelectedPostRemoved(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "J")
	f := sampleFeeds()[0]
	f.Posts = f.Posts[:1]
	m.refresh.Start([]string{sampleURL(0)})
	m, _ = update(m, feedLoadedMsg{feed: f})
	if m.main.posts.cursor != 0 || selectedTitle(m.main.posts) != "Alpha post 0" {
		t.Errorf("cursor should clamp, got %d %q", m.main.posts.cursor, selectedTitle(m.main.posts))
	}
}

func TestEmptyStates(t *testing.T) {
	cfg := config.Default()
	m := *NewModel(cfg, newLib(testDB(t)), nil, nil)
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if !strings.Contains(ansi.Strip(m.View().Content), "No feeds loaded yet") {
		t.Error("empty model should say no feeds")
	}
	m, _ = keys(m, "j", "k", "x", "X", "enter", "l", "enter", "o")
	empty := []rss.Feed{{Title: "Empty", URL: "https://e.example/feed", Posts: nil}}
	m, _ = update(m, loaded(m, empty...))
	if !strings.Contains(ansi.Strip(m.View().Content), "No posts") {
		t.Error("empty feed should say no posts")
	}
	m, _ = keys(m, "l", "x", "enter", "j", "X")
	if m.page.Shown() {
		t.Error("enter on an empty posts pane must not open the reader")
	}
}

func TestStatusText(t *testing.T) {
	now := t0
	tests := []struct {
		unread int
		r      *refresh.Refresher
		want   string
	}{
		{18, refreshAt(11, 3, time.Time{}), "refreshing 3/11…"},
		{18, refreshAt(0, 0, now.Add(-20*time.Second)), "18 unread · refreshed just now"},
		{18, refreshAt(0, 0, now.Add(-2*time.Minute)), "18 unread · refreshed 2m ago"},
		{1, refreshAt(0, 0, now.Add(-3*time.Hour)), "1 unread · refreshed 3h ago"},
		{0, refreshAt(0, 0, now.Add(-50*time.Hour)), "0 unread · refreshed 2d ago"},
		{5, refreshAt(0, 0, time.Time{}), "5 unread"},
	}
	for _, tt := range tests {
		if got := statusText(tt.unread, tt.r, now); got != tt.want {
			t.Errorf("statusText(%d) = %q, want %q", tt.unread, got, tt.want)
		}
	}
}

func TestMainView_FitsWidth(t *testing.T) {
	for _, w := range []int{20, 60, 69, 70, 120, 200} {
		m := newTestModel(t)
		m, _ = update(m, tea.WindowSizeMsg{Width: w, Height: 12})
		lines := strings.Split(m.View().Content, "\n")
		if len(lines) != 12 {
			t.Errorf("w=%d: %d lines, want 12", w, len(lines))
		}
		for i, l := range lines {
			if sw := ansi.StringWidth(l); sw > w {
				t.Errorf("w=%d line %d is %d wide", w, i, sw)
			}
		}
		twoPanes := strings.Contains(ansi.Strip(lines[0]), "│")
		if (w >= 70) != twoPanes {
			t.Errorf("w=%d: two panes = %v", w, twoPanes)
		}
		if last := ansi.Strip(lines[11]); w >= 60 && !strings.Contains(last, "unread") {
			t.Errorf("w=%d: status line missing: %q", w, last)
		}
	}
}

func TestQuit(t *testing.T) {
	if _, msgs := keys(newTestModel(t), "q"); !quits(msgs) {
		t.Error("q on the main screen should quit")
	}
}

// The status line's unread total follows read-state changes and refreshes.
func TestStatusLine_UnreadTracksChanges(t *testing.T) {
	m := newTestModel(t)
	status := func() string {
		lines := strings.Split(ansi.Strip(m.View().Content), "\n")
		return lines[len(lines)-1]
	}
	if s := status(); !strings.Contains(s, "3 unread") {
		t.Fatalf("status %q, want 3 unread", s)
	}
	m, _ = keys(m, "l", "x") // Alpha's first post, unread, becomes read
	if s := status(); !strings.Contains(s, "2 unread") {
		t.Errorf("after toggle: status %q, want 2 unread", s)
	}
	m, _ = update(m, feedLoadedMsg{feed: sampleFeeds()[1]}) // freshly parsed: no flags
	if s := status(); !strings.Contains(s, "2 unread") {
		t.Errorf("after refresh: status %q, want 2 unread (read state survives a refresh)", s)
	}
}

// "All posts" lists posts published at the same moment in feed order, then
// post order, as a stable sort by date does.
func TestAllPosts_TiesKeepFeedOrder(t *testing.T) {
	m := newTestModel(t)
	fs := sampleFeeds()
	for i := range fs {
		for j := range fs[i].Posts {
			fs[i].Posts[j].Published = t0
		}
	}
	m, _ = update(m, loaded(m, fs...))
	m.main.feeds.move(-m.main.feeds.cursor)
	m.main.rebuildPosts()
	var got []string
	for _, it := range m.main.posts.items {
		got = append(got, it.title)
	}
	var want []string
	for _, f := range fs {
		for _, p := range f.Posts {
			want = append(want, p.Title)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

// R while r is running must not fetch again the feeds r has finished: the
// round's total is the number of distinct feeds, not feeds plus repeats.
func TestRefresh_AllDuringSelectedSkipsFinished(t *testing.T) {
	m := newTestModel(t)
	f := sampleFeeds()
	m, _ = keys(m, "k") // All posts: r stands for every feed
	m, first := update(m, press("r"))
	if len(first) != 3 {
		t.Fatalf("r on All posts fetched %d feeds, want 3", len(first))
	}
	m, _ = update(m, feedLoadedMsg{feed: f[0]})
	m, _ = update(m, feedLoadedMsg{feed: f[1]})
	m, second := update(m, press("R"))
	if len(second) != 0 {
		t.Errorf("R refetched %d feeds already finished or in flight", len(second))
	}
	if done, total := m.refresh.Progress(); done != 2 || total != 3 {
		t.Errorf("progress %d/%d, want 2/3", done, total)
	}
	m, _ = update(m, feedLoadedMsg{feed: f[2]})
	if totalOf(m) != 0 {
		t.Error("the round should be over")
	}
}

// totalOf is the number of feeds in the running refresh.
func totalOf(m Model) int {
	_, total := m.refresh.Progress()
	return total
}

// refreshAt is a refresher part-way through a round of total feeds, done of
// them finished, whose last round ended at at.
func refreshAt(total, done int, at time.Time) *refresh.Refresher {
	r := refresh.New(rss.NewFeedSet(), fakeSource{})
	if !at.IsZero() {
		r.SetAt(at)
	}
	var urls []string
	for i := range total {
		urls = append(urls, fmt.Sprintf("https://f%d.example", i))
	}
	r.Start(urls)
	for _, u := range urls[:done] {
		r.Finish(rss.Feed{URL: u}, t0)
	}
	return r
}

// A feed with no title is named by its URL's host, without "www.", wherever
// it is shown.
func TestUntitledFeed_NamedByHost(t *testing.T) {
	m := newTestModel(t)
	noTicks(&m)
	feeds := sampleFeeds()
	feeds[1].Title = ""
	feeds[1].URL = "https://www.danluu.com/atom.xml"
	m, _ = update(m, loaded(m, feeds...))

	if got := m.main.feeds.items[2].title; got != "danluu.com" {
		t.Errorf("feeds pane: %q, want danluu.com", got)
	}
	m, _ = keys(m, "k") // All posts
	if got := m.main.posts.items[1].source; got != "danluu.com" {
		t.Errorf("All posts feed column: %q, want danluu.com", got)
	}
	m, _ = keys(m, "/")
	m = typeText(m, "beta post 0")
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "danluu.com") {
		t.Error("palette rows should name the feed by host")
	}
	m, _ = keys(m, "esc")
	m.openReader(rss.PostRef{FeedURL: feeds[1].URL, Key: "Beta-0"}) // the reader header names the feed the same way
	if hdr := ansi.Strip(strings.Split(m.View().Content, "\n")[0]); !strings.Contains(hdr, "danluu.com") {
		t.Errorf("reader header %q should name the feed by host", hdr)
	}
	m, _ = keys(m, "esc")

	m.refresh.Start([]string{sampleURL(1)})
	m, _ = update(m, feedLoadedMsg{feed: rss.Feed{URL: feeds[1].URL, Err: timeoutErr()}})
	if got := toastTexts(m); len(got) != 1 || got[0] != "danluu.com: timed out" {
		t.Errorf("toasts = %q", got)
	}
}

// The hint bar is a strip on the surface colour across the whole width, on
// both screens, with the keys still in the accent colour.
func TestHintBar_Strip(t *testing.T) {
	m := newTestModel(t)
	bg := ansi.Style{}.BackgroundColor(hex(m.cfg.Colors.Borders)).String()
	key := ansi.Style{}.ForegroundColor(hex(m.cfg.Colors.Accent)).String()
	for _, screen := range []string{"main", "reader"} {
		if screen == "reader" {
			m, _ = keys(m, "l", "enter")
		}
		lines := strings.Split(m.View().Content, "\n")
		bar := lines[len(lines)-1]
		if w := ansi.StringWidth(bar); w != m.width {
			t.Errorf("%s: bar is %d wide, want %d", screen, w, m.width)
		}
		if !strings.HasPrefix(bar, bg) || !strings.HasPrefix(ansi.Strip(bar), " ") {
			t.Errorf("%s: bar should start on the background with padding: %q", screen, bar)
		}
		if !strings.Contains(bar, key) {
			t.Errorf("%s: keys should keep the accent colour: %q", screen, bar)
		}
		// Every reset before the end must re-open the background.
		body := strings.TrimSuffix(bar, ansi.ResetStyle)
		for i := strings.Index(body, ansi.ResetStyle); i >= 0; i = strings.Index(body, ansi.ResetStyle) {
			body = body[i+len(ansi.ResetStyle):]
			if !strings.HasPrefix(body, bg) {
				t.Fatalf("%s: bar drops the background after a reset: %q", screen, bar)
			}
		}
	}
}

// newMain is a main page on its own over sampleFeeds: no Model, no messages.
func newMain(t *testing.T, home string) (mainPage, *library.Library) {
	t.Helper()
	lib := newLib(testDB(t))
	p := newMainPage(lib, home, "02/01/2006")
	p.Resize(120, 40)
	lib.Restore(library.Snapshot{Feeds: sampleFeeds()})
	p.Sync()
	return p, lib
}

// The posts pane shows whatever feed the cursor is on, however the cursor
// got there, and the page follows the library without being told.
func TestMainPage_PostsFollowFeedCursor(t *testing.T) {
	p, lib := newMain(t, "home")
	if got := selectedTitle(p.feeds); got != "Alpha" {
		t.Fatalf("home starts on the first feed, got %q", got)
	}
	if got := selectedTitle(p.posts); got != "Alpha post 0" {
		t.Errorf("posts pane shows %q", got)
	}
	p.Key(press("j"))
	if got := selectedTitle(p.posts); got != "Beta post 0" {
		t.Errorf("after moving to Beta the posts pane shows %q", got)
	}
	if _, ok := p.SelectedPost(); ok {
		t.Error("no post is selected while the feeds pane has focus")
	}
	p.Key(press("l"))
	e, ok := p.SelectedPost()
	if !ok || e.Ref.Key != "Beta-0" || e.Read {
		t.Fatalf("selected %+v, %v", e, ok)
	}

	if err := lib.SetRead(e.Ref, true); err != nil {
		t.Fatal(err)
	}
	p.Sync()
	if it, _ := p.posts.selected(); it.strong || it.marker != " " {
		t.Errorf("a read post should be drawn read: %+v", it)
	}
	if p.Unread() != 11 {
		t.Errorf("unread = %d, want 11", p.Unread())
	}
}

func TestMainPage_MixedHomeStartsOnAllPosts(t *testing.T) {
	p, _ := newMain(t, "mixed")
	if got := selectedTitle(p.feeds); got != "All posts" {
		t.Errorf("mixed starts on %q", got)
	}
	if got := p.SelectedFeeds(); len(got) != 3 {
		t.Errorf("All posts stands for every feed, got %q", got)
	}
}
