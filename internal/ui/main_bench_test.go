package ui

import (
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/rss"
	"github.com/isabelroses/izrss/internal/storage"
)

// thirdsUnread is a read status for every post of feeds: one in three unread.
func thirdsUnread(feeds rss.Feeds) []storage.PostReadStatus {
	var out []storage.PostReadStatus
	for _, f := range feeds {
		for j, p := range f.Posts {
			out = append(out, storage.PostReadStatus{FeedURL: f.URL, Key: p.Key(), Read: j%3 != 0})
		}
	}
	return out
}

func benchFeedsLarge() rss.Feeds {
	fs := make(rss.Feeds, 11)
	for i := range fs {
		posts := make([]rss.Post, 36)
		for j := range posts {
			posts[j] = rss.Post{UUID: fmt.Sprintf("%d-%d", i, j), Title: fmt.Sprintf("A reasonably long post title number %d from feed %d", j, i), Published: t0.Add(-time.Duration(j*11+i) * time.Hour), Date: "04/11/2026"}
		}
		fs[i] = rss.Feed{Title: fmt.Sprintf("Feed number %d", i), URL: fmt.Sprintf("https://f%d.example", i), Posts: posts}
	}
	return fs
}

func BenchmarkMainView(b *testing.B) {
	db := testDB(b)
	m := *NewModel(config.Default(), db, nil, nil, nil)
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = update(m, feedsRefreshedMsg{feeds: benchFeedsLarge()})
	b.ReportAllocs()
	for b.Loop() {
		_ = m.View().Content
	}
}

// benchModel is a ready 120x40 model showing benchFeedsLarge, whose read
// state is also saved in its database.
func benchModel(b *testing.B) Model {
	b.Helper()
	log.SetOutput(io.Discard) // notify logs every toast
	b.Cleanup(func() { log.SetOutput(os.Stderr) })
	db := testDB(b)
	cfg := config.Default()
	feeds := benchFeedsLarge()
	for _, f := range feeds {
		cfg.Urls = append(cfg.Urls, f.URL)
	}
	if err := db.SavePostReadStatuses(thirdsUnread(feeds)); err != nil {
		b.Fatal(err)
	}
	m := *NewModel(cfg, db, nil, nil, nil)
	m.now = func() time.Time { return t0 }
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = update(m, feedsRefreshedMsg{feeds: feeds})
	return m
}

// BenchmarkFeedSwitch moves the feeds-pane selection between "All posts"
// and the first feed, rebuilding the posts pane each time.
func BenchmarkFeedSwitch(b *testing.B) {
	m := benchModel(b)
	m.focus = paneFeeds
	b.ReportAllocs()
	for b.Loop() {
		m.moveFocused(-1)
		m.moveFocused(1)
	}
}

// BenchmarkFeedLoaded handles one feed's refresh result.
func BenchmarkFeedLoaded(b *testing.B) {
	m := benchModel(b)
	m.refresh.Start(m.feeds.URLs()) // keep the refresh "in progress"
	feed := m.feeds.All()[3]
	b.ReportAllocs()
	for b.Loop() {
		fresh := feed
		fresh.Posts = append([]rss.Post(nil), feed.Posts...)
		_ = m.feedLoaded(feedLoadedMsg{feed: fresh})
	}
}

// BenchmarkMainViewToast draws the main screen with a toast showing, which
// composites a layer over it.
func BenchmarkMainViewToast(b *testing.B) {
	m := benchModel(b)
	_ = m.notify(toastError, "Feed number 3: connection refused")
	b.ReportAllocs()
	for b.Loop() {
		_ = m.View().Content
	}
}
