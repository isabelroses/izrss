package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/library"
	"github.com/isabelroses/izrss/internal/rss"
)

func typeText(m Model, s string) Model {
	for _, r := range s {
		m, _ = update(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

func TestPalette_LiveFilter(t *testing.T) {
	m, _ := keys(newTestModel(t), "/")
	if m.overlay != overlayPalette {
		t.Fatal("/ should open the palette")
	}
	m = typeText(m, "beta")
	if len(m.palette.hits) != 4 {
		t.Errorf("want 4 hits for beta, got %d", len(m.palette.hits))
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "4 results") {
		t.Error("palette should show the result count")
	}
}

func TestPalette_EnterOpensReader(t *testing.T) {
	m, _ := keys(newTestModel(t), "/")
	m = typeText(m, "gamma post 2")
	m, _ = keys(m, "enter")
	if m.overlay != overlayNone || !m.page.Shown() || m.page.post.Title != "Gamma post 2" {
		t.Errorf("enter: overlay %v reader %v post %q", m.overlay, m.page.Shown(), m.page.post.Title)
	}
}

func TestPalette_EscRestores(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "j", "j")
	before := selectedTitle(m.main.posts)
	m, _ = keys(m, "/")
	m = typeText(m, "alpha")
	m, _ = keys(m, "down", "esc")
	if m.overlay != overlayNone || m.page.Shown() || selectedTitle(m.main.posts) != before || m.main.focus != panePosts {
		t.Errorf("esc should restore: overlay %v reader %v post %q", m.overlay, m.page.Shown(), selectedTitle(m.main.posts))
	}
}

func TestPalette_SwallowsKeys(t *testing.T) {
	m, _ := keys(newTestModel(t), "/")
	unread := m.main.unread
	var msgs []tea.Msg
	m, msgs = keys(m, "q", "?", "x")
	if quits(msgs) || m.overlay != overlayPalette || m.main.unread != unread {
		t.Errorf("palette must swallow keys: quit %v overlay %v", quits(msgs), m.overlay)
	}
	if v := m.palette.input.Value(); v != "q?x" {
		t.Errorf("keys should be typed, got %q", v)
	}
}

func TestPalette_NoResults(t *testing.T) {
	m, _ := keys(newTestModel(t), "/")
	m = typeText(m, "zzz")
	if !strings.Contains(ansi.Strip(m.View().Content), "No results") {
		t.Error("should say No results")
	}
	m, _ = keys(m, "enter")
	if m.page.Shown() || m.overlay != overlayPalette {
		t.Error("enter with no results is a no-op")
	}
}

func TestPalette_WidthAndCap(t *testing.T) {
	m := newTestModel(t)
	many := rss.Feed{Title: "Many", URL: "https://many.example/feed"}
	for i := range 50 {
		many.Posts = append(many.Posts, rss.Post{UUID: fmt.Sprint(i), Title: fmt.Sprintf("match %d", i), Content: "match"})
	}
	m.lib.Replace(many)
	m, _ = update(m, tea.WindowSizeMsg{Width: 200, Height: 40})
	m, _ = keys(m, "/")
	m = typeText(m, "match")
	box := m.palette.View(m.width, m.styles)
	lines := strings.Split(box, "\n")
	if w := ansi.StringWidth(lines[0]); w != 80 {
		t.Errorf("box width %d, want 80", w)
	}
	rows := 0
	for _, l := range lines {
		if strings.Contains(ansi.Strip(l), "Many") { // result rows show the feed name
			rows++
		}
	}
	if rows != 10 {
		t.Errorf("want 10 result rows, got %d", rows)
	}
}

func TestHelpOverlay(t *testing.T) {
	m, _ := keys(newTestModel(t), "?")
	if m.overlay != overlayHelp {
		t.Fatal("? should open help")
	}
	v := ansi.Strip(m.View().Content)
	for _, want := range []string{"search", "quit", "refresh"} {
		if !strings.Contains(v, want) {
			t.Errorf("help lacks %q", want)
		}
	}
	m, msgs := keys(m, "q")
	if m.overlay != overlayNone || quits(msgs) {
		t.Error("any key should close help without acting")
	}
}

func TestOverlay_ComposedOverBase(t *testing.T) {
	m, _ := keys(newTestModel(t), "/")
	lines := strings.Split(m.View().Content, "\n")
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > 120 {
			t.Errorf("line %d is %d wide", i, w)
		}
	}
	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "All posts") {
		t.Error("the panes should stay visible around the palette")
	}
}

// Feeds shrinking under an open palette must not leave hits pointing past
// the end of a feed's posts.
func TestPalette_FeedShrinksWhileOpen(t *testing.T) {
	m, _ := keys(newTestModel(t), "/")
	m = typeText(m, "gamma post")
	m, _ = keys(m, "down", "down", "down") // cursor on the last Gamma hit
	shrunk := sampleFeeds()[2]
	shrunk.Posts = shrunk.Posts[:1]
	m, _ = update(m, feedLoadedMsg{feed: shrunk})
	if len(m.palette.hits) != 1 {
		t.Errorf("hits should follow the new feeds: got %d, want 1", len(m.palette.hits))
	}
	_ = m.View()
	m, _ = keys(m, "enter")
	if !m.page.Shown() || m.page.post.Title != "Gamma post 0" {
		t.Errorf("enter: reader %v post %q", m.page.Shown(), m.page.post.Title)
	}
}

// A hit whose post is gone (its feed shrank or was dropped after the hit was
// listed) opens nothing, and does not crash or open some other post.
func TestPalette_EnterOnStaleHit(t *testing.T) {
	m, _ := keys(newTestModel(t), "/")
	m = typeText(m, "gamma post")
	m, _ = keys(m, "down", "down", "down") // the last Gamma hit
	hit := m.palette.hits[m.palette.cursor]
	shrunk := sampleFeeds()[2]
	shrunk.Posts = shrunk.Posts[:1]
	m.lib.Replace(shrunk) // behind the palette's back, as a refresh landing mid-keypress would
	m.palette.hits = []rss.PostRef{hit}
	m.palette.cursor = 0
	m, _ = keys(m, "enter")
	if m.page.Shown() || m.overlay != overlayPalette {
		t.Errorf("a stale hit should do nothing: reader %v overlay %v", m.page.Shown(), m.overlay)
	}
	m.lib.Restore(library.Snapshot{})
	m, _ = keys(m, "enter")
	_ = m.View()
}

// An open palette keeps its results current as the library changes, but
// waits for a refresh round to finish rather than searching per feed.
func TestPalette_FollowsLibrary(t *testing.T) {
	lib := newLib(testDB(t))
	lib.Restore(library.Snapshot{Feeds: sampleFeeds()})
	p := newPalette(lib)
	p.Open()
	for _, r := range "alpha post" {
		p.Key(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if len(p.hits) != 4 {
		t.Fatalf("hits = %d, want Alpha's 4", len(p.hits))
	}
	f := sampleFeeds()[0]
	f.Posts = f.Posts[:1]
	lib.Replace(f)
	p.Sync(false)
	if len(p.hits) != 4 {
		t.Error("mid-refresh the palette should wait")
	}
	p.Sync(true)
	if len(p.hits) != 1 {
		t.Errorf("after the refresh hits = %d, want 1", len(p.hits))
	}
}
