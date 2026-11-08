package ui

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/rss"
	"github.com/isabelroses/izrss/internal/storage"
)

// A long feed list: 250 feeds of 40 posts, 10,000 posts in all.
const (
	scaleFeeds = 250
	scalePosts = 40
)

var readerSink string

// thirdsUnread is a read status for every post of feeds: one in three unread.
func thirdsUnread(feeds []rss.Feed) []storage.PostReadStatus {
	var out []storage.PostReadStatus
	for _, f := range feeds {
		for j, p := range f.Posts {
			out = append(out, storage.PostReadStatus{FeedURL: f.URL, Key: p.Key(), Read: j%3 != 0})
		}
	}
	return out
}

// scaleFeedXML is an RSS 2.0 document as a feed server sends it: 40 items
// of ~2KB HTML each, published an hour apart and offset per feed so "All
// posts" interleaves them.
func scaleFeedXML(i int) string {
	body := strings.Repeat("&lt;p&gt;The quick brown fox — “jumps” over the lazy dog’s back.&lt;/p&gt;\n", 30)
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>Feed number %d</title><link>https://f%d.example/</link>`, i, i)
	for j := range scalePosts {
		pub := t0.Add(-time.Duration(j*scaleFeeds+i) * time.Minute).Format(time.RFC1123Z)
		fmt.Fprintf(&b, `<item><title>A reasonably long post title number %d from feed %d</title><link>https://f%d.example/%d</link><guid>f%d-%d</guid><pubDate>%s</pubDate><description>%s</description></item>`,
			j, i, i, j, i, j, pub, body)
	}
	b.WriteString(`</channel></rss>`)
	return b.String()
}

// scaleSetup fills a database with 250 feeds, fetched and cached, and read
// state for every post (two in three read), returning a config listing them.
func scaleSetup(b *testing.B) (*config.Config, *storage.DB) {
	b.Helper()
	log.SetOutput(io.Discard)
	b.Cleanup(func() { log.SetOutput(os.Stderr) })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var i int
		_, _ = fmt.Sscanf(r.URL.Path, "/f%d.xml", &i)
		_, _ = io.WriteString(w, scaleFeedXML(i))
	}))
	b.Cleanup(srv.Close)
	db := testDB(b)
	cfg := config.Default()
	for i := range scaleFeeds {
		cfg.Urls = append(cfg.Urls, fmt.Sprintf("%s/f%d.xml", srv.URL, i))
	}
	feeds := rss.NewFetcher(rss.NewStore(db)).FetchAll(cfg.Urls)
	for i := range feeds {
		if feeds[i].Err != nil || feeds[i].CacheErr != nil || len(feeds[i].Posts) != scalePosts {
			b.Fatalf("feed %d: %d posts, err %v", i, len(feeds[i].Posts), feeds[i].Err)
		}
	}
	if err := db.SavePostReadStatuses(thirdsUnread(feeds)); err != nil {
		b.Fatal(err)
	}
	return cfg, db
}

// scaleModel is a ready 120x40 model showing the 250 cached feeds.
func scaleModel(b *testing.B) Model {
	b.Helper()
	cfg, db := scaleSetup(b)
	// A real source, so refresh.Start runs a round; the benchmark delivers
	// the results itself and never runs the fetches.
	m := *NewModel(cfg, newLib(db), rss.NewFetcher(rss.NewStore(db)), nil)
	m.now = func() time.Time { return t0 }
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = update(m, m.loadCachedFeeds()())
	return m
}

// BenchmarkStartup250 is time to first frame with 250 cached feeds: the
// model, the cache-only load with read state, and the first View.
func BenchmarkStartup250(b *testing.B) {
	cfg, db := scaleSetup(b)
	b.ReportAllocs()
	for b.Loop() {
		m := *NewModel(cfg, newLib(db), nil, nil)
		m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
		m, _ = update(m, m.loadCachedFeeds()())
		readerSink = m.View().Content
	}
}

// BenchmarkRefreshAll250 delivers a full refresh's 250 feedLoadedMsgs one by
// one, drawing a frame after each as the program does.
func BenchmarkRefreshAll250(b *testing.B) {
	b.Run("idle", func(b *testing.B) { benchRefreshAll(b, "") })
	// The palette is open on a query, so every result re-runs the search.
	b.Run("searching", func(b *testing.B) { benchRefreshAll(b, "fox") })
}

func benchRefreshAll(b *testing.B, query string) {
	m := scaleModel(b)
	if query != "" {
		m, _ = update(m, press("/"))
		for _, r := range query {
			m, _ = update(m, tea.KeyPressMsg{Code: r, Text: string(r)})
		}
	}
	fresh := make([]rss.Feed, m.lib.Len())
	for i := range fresh {
		f := feedAt(m, i)
		fresh[i] = f
		fresh[i].Posts = append([]rss.Post(nil), f.Posts...)
	}
	b.ReportAllocs()
	for b.Loop() {
		m.refresh.Start(m.lib.URLs())
		for _, f := range fresh {
			f.Posts = append([]rss.Post(nil), f.Posts...)
			m, _ = update(m, feedLoadedMsg{feed: f})
			readerSink = m.View().Content
		}
	}
}

// BenchmarkMainView250 draws the main screen over 10k posts, with one feed
// and with "All posts" selected.
func BenchmarkMainView250(b *testing.B) {
	for _, tc := range []struct {
		name string
		row  int
	}{{"feed", 1}, {"all", 0}} {
		b.Run(tc.name, func(b *testing.B) {
			m := scaleModel(b)
			m.main.feeds.move(tc.row - m.main.feeds.cursor)
			m.main.rebuildPosts()
			b.ReportAllocs()
			for b.Loop() {
				readerSink = m.View().Content
			}
		})
	}
}

// BenchmarkPaletteTyping250 types a query into the palette over 10k posts,
// drawing a frame after each keystroke.
func BenchmarkPaletteTyping250(b *testing.B) {
	m := scaleModel(b)
	b.ReportAllocs()
	for b.Loop() {
		m, _ = update(m, press("/"))
		for _, r := range "fox jumps" {
			m, _ = update(m, tea.KeyPressMsg{Code: r, Text: string(r)})
			readerSink = m.View().Content
		}
		m, _ = update(m, press("esc"))
	}
}
