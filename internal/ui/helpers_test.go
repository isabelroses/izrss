package ui

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/rss"
	"github.com/isabelroses/izrss/internal/storage"
)

var t0 = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

// sampleFeeds: three feeds of four posts; post j of feed i is j+3*i hours
// old, so "All posts" interleaves them. Post 0 of each feed is unread.
func sampleFeeds() rss.Feeds {
	names := []string{"Alpha", "Beta", "Gamma"}
	fs := make(rss.Feeds, len(names))
	for i, name := range names {
		url := fmt.Sprintf("https://%s.example/feed", name)
		posts := make([]rss.Post, 4)
		for j := range posts {
			posts[j] = rss.Post{
				UUID:      fmt.Sprintf("%s-%d", name, j),
				Title:     fmt.Sprintf("%s post %d", name, j),
				Link:      fmt.Sprintf("https://%s.example/%d", name, j),
				Content:   fmt.Sprintf("<p>%s body %d</p>", name, j),
				Published: t0.Add(-time.Duration(j*3+i) * time.Hour),
				Date:      "04/11/2026",
				Read:      j != 0,
			}
		}
		fs[i] = rss.Feed{Title: name, URL: url, Posts: posts}
	}
	return fs
}

func testDB(t testing.TB) *storage.DB {
	t.Helper()
	db, err := storage.New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// newTestModel is a ready 120x40 model showing sampleFeeds, feeds pane
// focused on the first real feed.
func newTestModel(t *testing.T) Model {
	t.Helper()
	cfg := config.Default()
	for _, f := range sampleFeeds() {
		cfg.Urls = append(cfg.Urls, f.URL)
	}
	m := *NewModel(cfg, testDB(t), nil, nil, nil)
	m.now = func() time.Time { return t0 }
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = update(m, feedsRefreshedMsg{feeds: sampleFeeds()})
	return m
}

func press(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func keys(m Model, ks ...string) (Model, []tea.Msg) {
	var all []tea.Msg
	for _, k := range ks {
		var msgs []tea.Msg
		m, msgs = update(m, press(k))
		all = append(all, msgs...)
	}
	return m, all
}

func selectedTitle(l list) string {
	it, _ := l.selected()
	return it.title
}

// quits reports whether msgs contain tea.QuitMsg.
func quits(msgs []tea.Msg) bool {
	_, ok := find[tea.QuitMsg](msgs)
	return ok
}
