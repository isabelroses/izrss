package ui

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/rss"
)

func TestStartsOnFirstFeed(t *testing.T) {
	m := newTestModel(t)
	if m.screen != screenMain || m.focus != paneFeeds || selectedTitle(m.feedList) != "Alpha" {
		t.Errorf("start: screen %v focus %v feed %q", m.screen, m.focus, selectedTitle(m.feedList))
	}
	if selectedTitle(m.postList) != "Alpha post 0" || len(m.postList.items) != 4 {
		t.Errorf("posts pane should show Alpha's posts, got %q (%d)", selectedTitle(m.postList), len(m.postList.items))
	}
}

func TestPostsFollowFeedSelection(t *testing.T) {
	m, _ := keys(newTestModel(t), "j")
	if selectedTitle(m.feedList) != "Beta" || m.postList.items[0].title != "Beta post 0" {
		t.Errorf("posts should follow the feed: feed %q first post %q", selectedTitle(m.feedList), m.postList.items[0].title)
	}
}

func TestAllPostsFirstAndMixed(t *testing.T) {
	m, _ := keys(newTestModel(t), "k")
	if m.feedList.items[0].title != "All posts" || selectedTitle(m.feedList) != "All posts" {
		t.Fatalf("row 0 should be All posts, got %q", m.feedList.items[0].title)
	}
	if n := len(m.postList.items); n != 12 {
		t.Fatalf("All posts should list 12, got %d", n)
	}
	want := []string{"Alpha post 0", "Beta post 0", "Gamma post 0", "Alpha post 1"}
	for i, w := range want {
		if m.postList.items[i].title != w {
			t.Errorf("row %d = %q, want %q (newest first)", i, m.postList.items[i].title, w)
		}
	}
	if r := m.postList.items[1].rightText(); r != "Beta · 04/11/2026" {
		t.Errorf("All posts rows should name the feed: %q", r)
	}
	if m.feedList.items[0].right != "3" {
		t.Errorf("All posts unread count = %q, want 3", m.feedList.items[0].right)
	}
}

func TestHomeMixedSelectsAllPosts(t *testing.T) {
	cfg := config.Default()
	cfg.Home = "mixed"
	m := *NewModel(cfg, testDB(t), nil, nil, nil)
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = update(m, feedsRefreshedMsg{feeds: sampleFeeds()})
	if selectedTitle(m.feedList) != "All posts" {
		t.Errorf("home = mixed should start on All posts, got %q", selectedTitle(m.feedList))
	}
}

func TestPaneSwitch(t *testing.T) {
	m := newTestModel(t)
	for _, k := range []string{"l", "right", "tab"} {
		m.focus = paneFeeds
		if m, _ = keys(m, k); m.focus != panePosts {
			t.Errorf("%s should focus posts", k)
		}
	}
	for _, k := range []string{"h", "left"} {
		m.focus = panePosts
		if m, _ = keys(m, k); m.focus != paneFeeds {
			t.Errorf("%s should focus feeds", k)
		}
	}
	m.focus = paneFeeds
	if m, _ = keys(m, "enter"); m.focus != panePosts || m.screen != screenMain {
		t.Error("enter on feeds should focus posts, not open the reader")
	}
}

func TestToggleReadFromAllPosts(t *testing.T) {
	m, _ := keys(newTestModel(t), "k", "l", "j") // All posts, row 1 = Beta post 0 (unread)
	if selectedTitle(m.postList) != "Beta post 0" {
		t.Fatalf("selected %q", selectedTitle(m.postList))
	}
	m, _ = keys(m, "x")
	if !m.feeds[1].Posts[0].Read {
		t.Error("Beta post 0 should be read")
	}
	for i, f := range m.feeds {
		for j, p := range f.Posts {
			if i != 1 && j == 0 && p.Read {
				t.Errorf("%s was changed too", p.Title)
			}
		}
	}
}

func TestMarkFeedRead(t *testing.T) {
	m, _ := keys(newTestModel(t), "X")
	if m.feeds.GetTotalUnreads() != 2 || m.feeds[0].GetTotalUnreads() != 0 {
		t.Errorf("X should mark only Alpha read; unread = %d", m.feeds.GetTotalUnreads())
	}
	m, _ = keys(m, "k", "X")
	if m.feeds.GetTotalUnreads() != 0 {
		t.Errorf("X on All posts should mark everything read")
	}
}

func TestKeys_JumpRefreshOpen(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "J")
	if m.postList.cursor != 3 { // 4 posts: jump 5 clamps at 3
		t.Errorf("J should jump, cursor %d", m.postList.cursor)
	}
	m, _ = keys(m, "K")
	if m.postList.cursor != 0 {
		t.Errorf("K should jump back, cursor %d", m.postList.cursor)
	}

	var opened string
	m.openURL = func(u string) error { opened = u; return nil }
	m, _ = keys(m, "o")
	if opened != "https://Alpha.example/0" {
		t.Errorf("o opened %q", opened)
	}

	m.fetcher = rss.NewFetcher(m.db, "02/01/2006")
	if r, _ := update(m, press("r")); r.refresh.total != 1 {
		t.Errorf("r on a feed should refresh 1 feed, got %d", r.refresh.total)
	}
	if r, _ := update(m, press("R")); r.refresh.total != 3 {
		t.Errorf("R should refresh all 3, got %d", r.refresh.total)
	}
}

func TestRefresh_KeepsSelectionByKey(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "j", "j") // Alpha post 2
	f := sampleFeeds()[0]
	f.Posts = append([]rss.Post{{UUID: "new1", Title: "new 1", Published: t0.Add(time.Hour)}, {UUID: "new2", Title: "new 2", Published: t0.Add(time.Hour)}}, f.Posts...)
	m.refresh = refreshing(0)
	m, _ = update(m, feedLoadedMsg{feed: f, index: 0})
	if selectedTitle(m.postList) != "Alpha post 2" {
		t.Errorf("selection moved to %q", selectedTitle(m.postList))
	}
}

func TestRefresh_SelectedPostRemoved(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "J")
	f := sampleFeeds()[0]
	f.Posts = f.Posts[:1]
	m.refresh = refreshing(0)
	m, _ = update(m, feedLoadedMsg{feed: f, index: 0})
	if m.postList.cursor != 0 || selectedTitle(m.postList) != "Alpha post 0" {
		t.Errorf("cursor should clamp, got %d %q", m.postList.cursor, selectedTitle(m.postList))
	}
}

func TestEmptyStates(t *testing.T) {
	cfg := config.Default()
	m := *NewModel(cfg, testDB(t), nil, nil, nil)
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if !strings.Contains(ansi.Strip(m.View().Content), "No feeds loaded yet") {
		t.Error("empty model should say no feeds")
	}
	m, _ = keys(m, "j", "k", "x", "X", "enter", "l", "enter", "o")
	empty := rss.Feeds{{Title: "Empty", URL: "https://e.example/feed", Posts: nil}}
	m, _ = update(m, feedsRefreshedMsg{feeds: empty})
	if !strings.Contains(ansi.Strip(m.View().Content), "No posts") {
		t.Error("empty feed should say no posts")
	}
	m, _ = keys(m, "l", "x", "enter", "j", "X")
	if m.screen != screenMain {
		t.Error("enter on an empty posts pane must not open the reader")
	}
}

func TestStatusText(t *testing.T) {
	now := t0
	tests := []struct {
		unread int
		r      refreshState
		want   string
	}{
		{18, refreshState{total: 11, done: 3}, "refreshing 3/11…"},
		{18, refreshState{at: now.Add(-20 * time.Second)}, "18 unread · refreshed just now"},
		{18, refreshState{at: now.Add(-2 * time.Minute)}, "18 unread · refreshed 2m ago"},
		{1, refreshState{at: now.Add(-3 * time.Hour)}, "1 unread · refreshed 3h ago"},
		{0, refreshState{at: now.Add(-50 * time.Hour)}, "0 unread · refreshed 2d ago"},
		{5, refreshState{}, "5 unread"},
	}
	for _, tt := range tests {
		if got := statusText(tt.unread, tt.r, now); got != tt.want {
			t.Errorf("statusText(%d, %+v) = %q, want %q", tt.unread, tt.r, got, tt.want)
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
	fresh := sampleFeeds()[1]
	fresh.Posts[1].Read = false // not saved read, so the refresh leaves it unread
	m, _ = update(m, feedLoadedMsg{feed: fresh, index: 1})
	if s := status(); !strings.Contains(s, "3 unread") {
		t.Errorf("after refresh: status %q, want 3 unread", s)
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
	m, _ = update(m, feedsRefreshedMsg{feeds: fs})
	m.feedList.move(-m.feedList.cursor)
	m.rebuildPosts()
	var got []string
	for _, it := range m.postList.items {
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

// sortNewestFirst orders exactly as a stable sort by descending time does,
// whether the input is in runs (as feeds are) or not.
func TestSortNewestFirst_MatchesStableSort(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for n := range 200 {
		refs := make([]postRef, n)
		for i := range refs {
			// Few distinct times so ties are common; some zero times.
			ts := time.Time{}
			if rng.IntN(5) > 0 {
				ts = t0.Add(time.Duration(rng.IntN(8)) * time.Second / 2)
			}
			refs[i] = postRef{sec: ts.Unix(), nsec: int32(ts.Nanosecond()), feed: int32(i)}
		}
		if n%2 == 0 { // feed-like: sorted runs
			for lo := 0; lo < n; lo += 7 {
				run := refs[lo:min(lo+7, n)]
				slices.SortStableFunc(run, func(a, b postRef) int { return cmpRef(b, a) })
			}
		}
		want := slices.Clone(refs)
		slices.SortStableFunc(want, func(a, b postRef) int { return cmpRef(b, a) })
		if got := sortNewestFirst(slices.Clone(refs)); !slices.Equal(got, want) {
			t.Fatalf("n=%d:\n got %v\nwant %v", n, got, want)
		}
	}
}

func cmpRef(a, b postRef) int {
	switch {
	case a.newer(b):
		return 1
	case b.newer(a):
		return -1
	}
	return 0
}

// A refresh requested while one is running only adds feeds not already in
// flight, so progress never counts past the number of feeds.
func TestRefresh_NoStacking(t *testing.T) {
	m := newTestModel(t)
	m.fetcher = rss.NewFetcher(m.db, "02/01/2006")
	m, first := update(m, press("r"))  // Alpha
	m, second := update(m, press("R")) // adds Beta and Gamma
	if m.refresh.total != len(m.feeds) {
		t.Fatalf("r then R: total %d, want %d", m.refresh.total, len(m.feeds))
	}
	if n := len(first) + len(second); n != len(m.feeds) {
		t.Errorf("r then R fetched %d feeds, want %d", n, len(m.feeds))
	}
	m, again := update(m, press("R"))
	if m.refresh.total != len(m.feeds) || len(again) != 0 {
		t.Errorf("R while all are in flight: total %d, %d fetches", m.refresh.total, len(again))
	}

	f := sampleFeeds()
	m, _ = update(m, feedLoadedMsg{feed: f[0], index: 0})
	m, _ = update(m, feedLoadedMsg{feed: f[0], index: 0}) // a late duplicate
	if m.refresh.done != 1 {
		t.Errorf("a duplicate result was counted: done %d", m.refresh.done)
	}
	if s := statusText(m.unread, m.refresh, m.now()); s != "refreshing 1/3…" {
		t.Errorf("status %q", s)
	}
	m, _ = update(m, feedLoadedMsg{feed: f[1], index: 1})
	m, _ = update(m, feedLoadedMsg{feed: f[2], index: 2})
	if m.refresh.total != 0 || m.refresh.at.IsZero() {
		t.Errorf("refresh should be finished: %+v", m.refresh)
	}
	m, msgs := update(m, press("r"))
	if m.refresh.total != 1 || len(msgs) != 1 {
		t.Errorf("r after the refresh finished: total %d, %d fetches", m.refresh.total, len(msgs))
	}
}

// A feed with no title is named by its URL's host, without "www.", wherever
// it is shown.
func TestUntitledFeed_NamedByHost(t *testing.T) {
	m := newTestModel(t)
	noTicks(&m)
	feeds := sampleFeeds()
	feeds[1].Title = ""
	feeds[1].URL = "https://www.danluu.com/atom.xml"
	m, _ = update(m, feedsRefreshedMsg{feeds: feeds})

	if got := m.feedList.items[2].title; got != "danluu.com" {
		t.Errorf("feeds pane: %q, want danluu.com", got)
	}
	m, _ = keys(m, "k") // All posts
	if got := m.postList.items[1].source; got != "danluu.com" {
		t.Errorf("All posts feed column: %q, want danluu.com", got)
	}
	m, _ = keys(m, "/")
	m = typeText(m, "beta post 0")
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "danluu.com") {
		t.Error("palette rows should name the feed by host")
	}
	m, _ = keys(m, "esc")

	m.refresh = refreshing(1)
	m, _ = update(m, feedLoadedMsg{feed: rss.Feed{URL: feeds[1].URL, Err: timeoutErr()}, index: 1})
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
