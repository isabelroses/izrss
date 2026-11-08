package ui

import (
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
	if !strings.Contains(m.postList.items[1].right, "Beta") {
		t.Errorf("All posts rows should name the feed: %q", m.postList.items[1].right)
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
	_, msgs := update(m, press("r"))
	if started, ok := find[refreshStartedMsg](msgs); !ok || started.total != 1 {
		t.Errorf("r on a feed should refresh 1 feed, got %+v", started)
	}
	_, msgs = update(m, press("R"))
	if started, _ := find[refreshStartedMsg](msgs); started.total != 3 {
		t.Errorf("R should refresh all 3, got %d", started.total)
	}
}

func TestRefresh_KeepsSelectionByKey(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "j", "j") // Alpha post 2
	f := sampleFeeds()[0]
	f.Posts = append([]rss.Post{{UUID: "new1", Title: "new 1", Published: t0.Add(time.Hour)}, {UUID: "new2", Title: "new 2", Published: t0.Add(time.Hour)}}, f.Posts...)
	m, _ = update(m, refreshStartedMsg{total: 1})
	m, _ = update(m, feedLoadedMsg{feed: f, index: 0})
	if selectedTitle(m.postList) != "Alpha post 2" {
		t.Errorf("selection moved to %q", selectedTitle(m.postList))
	}
}

func TestRefresh_SelectedPostRemoved(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "J")
	f := sampleFeeds()[0]
	f.Posts = f.Posts[:1]
	m, _ = update(m, refreshStartedMsg{total: 1})
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
