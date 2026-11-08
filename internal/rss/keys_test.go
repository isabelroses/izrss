package rss

import (
	"bytes"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/isabelroses/izrss/internal/storage"
)

func TestPostKey(t *testing.T) {
	tests := []struct {
		post Post
		want string
	}{
		{Post{UUID: "u", Link: "l", Title: "t"}, "u"},
		{Post{Link: "l", Title: "t"}, "l"},
		{Post{Title: "t", RawDate: "d"}, "t\x00d"},
		{Post{Title: "t", Published: time.Date(2026, 3, 4, 23, 0, 0, 0, time.UTC)}, "t\x0004/03/2026"},
	}
	for _, tt := range tests {
		if got := tt.post.Key(); got != tt.want {
			t.Errorf("Key(%+v) = %q, want %q", tt.post, got, tt.want)
		}
	}
}

// newFetcher is a Fetcher caching to a fresh database.
func newFetcher(t *testing.T) (*Fetcher, *storage.DB) {
	t.Helper()
	db := testDB(t)
	return NewFetcher(NewStore(db)), db
}

func testDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.New(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

const rssBody = `<?xml version="1.0"?><rss version="2.0"><channel><title>T</title>
<item><title>one</title><guid>g1</guid><description>d</description></item></channel></rss>`

func TestFetch_ErrorFields(t *testing.T) {
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prev) })

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(bad.Close)
	f, _ := newFetcher(t)
	feed := f.Fetch(bad.URL)
	var status *StatusError
	if !errors.As(feed.Err, &status) || status.Code != http.StatusInternalServerError {
		t.Errorf("a 500 should set a StatusError, got %v", feed.Err)
	}
	if feed.Name() != strings.TrimPrefix(bad.URL, "http://") {
		t.Errorf("a failed feed should be named by its host, got %q", feed.Name())
	}

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(rssBody))
	}))
	t.Cleanup(good.Close)
	f, db := newFetcher(t)
	_ = db.Close() // cache writes now fail
	feed = f.Fetch(good.URL)
	if feed.Err != nil || feed.CacheErr == nil || len(feed.Posts) != 1 {
		t.Errorf("want posts with CacheErr, got err=%v cacheErr=%v posts=%d", feed.Err, feed.CacheErr, len(feed.Posts))
	}
	if logs.Len() != 0 {
		t.Errorf("rss must not log, got %q", logs.String())
	}
}

func TestFetcher_BoundedConcurrency(t *testing.T) {
	var inflight, peak atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := inflight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		inflight.Add(-1)
		_, _ = w.Write([]byte(rssBody))
	}))
	t.Cleanup(srv.Close)
	f, _ := newFetcher(t)
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			f.Fetch(srv.URL + "/" + string(rune('a'+i)))
		})
	}
	wg.Wait()
	if p := peak.Load(); p > maxConcurrentFetches {
		t.Errorf("peak in-flight fetches %d > %d", p, maxConcurrentFetches)
	}
}

// Titles reach the TUI's panes and the reader header verbatim; control
// characters in them would be interpreted by the terminal.
func TestFetch_SanitizesTitles(t *testing.T) {
	// XML forbids these characters, but JSON Feed (also parsed) allows them.
	body := `{"version":"https://jsonfeed.org/version/1.1","title":"Evil\u001b]0;pwned\u0007 feed",
"items":[{"id":"g","title":"clear\u001b[2J me","content_text":"d"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	f, _ := newFetcher(t)
	feed := f.Fetch(srv.URL)
	if feed.Err != nil {
		t.Fatal(feed.Err)
	}
	for _, s := range []string{feed.Title, feed.Posts[0].Title} {
		if strings.ContainsAny(s, "\x1b\x07") {
			t.Errorf("control characters survived: %q", s)
		}
	}
	if feed.Title != "Evil]0;pwned feed" || feed.Posts[0].Title != "clear[2J me" {
		t.Errorf("titles = %q / %q", feed.Title, feed.Posts[0].Title)
	}
}

// A fetched feed comes back from the store as it was, without a parse; a
// cache from before the store kept feeds parsed still loads.
func TestStore_LoadsWhatFetchSaved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>T</title>
<item><title>dated</title><guid>g1</guid><pubDate>Wed, 04 Mar 2026 23:00:00 +0100</pubDate><description>d</description></item>
<item><title>undated</title><guid>g2</guid><pubDate>sometime</pubDate><description>e</description></item>
</channel></rss>`))
	}))
	t.Cleanup(srv.Close)
	path := t.TempDir() + "/t.db"
	db, err := storage.New(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fetched := NewFetcher(NewStore(db)).Fetch(srv.URL)
	if fetched.Err != nil || fetched.CacheErr != nil || fetched.Fetched.IsZero() {
		t.Fatalf("fetch: %+v", fetched)
	}
	// An old-format row: the feed's own XML, marked unparsed.
	legacy := "https://legacy.example/feed"
	if err := db.SaveFeedCache(legacy, []byte(rssBody)); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	if _, err := raw.Exec(`UPDATE feed_cache SET parsed = 0 WHERE url = ?`, legacy); err != nil {
		t.Fatal(err)
	}

	got := NewStore(db).Load([]string{srv.URL, legacy, "https://cold.example/feed"})
	loaded := got[0]
	if loaded.Err != nil || loaded.Title != "T" || len(loaded.Posts) != 2 || loaded.Fetched.IsZero() {
		t.Fatalf("loaded: %+v", loaded)
	}
	for i, p := range loaded.Posts {
		want := fetched.Posts[i]
		if p.Key() != want.Key() || !p.Published.Equal(want.Published) || p.RawDate != want.RawDate || p.Content != want.Content {
			t.Errorf("post %d: got %+v, want %+v", i, p, want)
		}
	}
	const layout = "02/01/2006 15:04 -0700"
	if got, want := loaded.Posts[0].Date(layout), fetched.Posts[0].Date(layout); got != want {
		t.Errorf("the date shows %q after the store, %q before", got, want)
	}
	if d := loaded.Posts[1].Date("2006"); d != "sometime" {
		t.Errorf("an unreadable date shows as the feed wrote it: %q", d)
	}
	if got[1].Err != nil || got[1].Title != "T" || len(got[1].Posts) != 1 {
		t.Errorf("legacy row: %+v", got[1])
	}
	cold := got[2]
	if cold.Title != "" || cold.Name() != "cold.example" || len(cold.Posts) != 0 || cold.Err != nil || !cold.Fetched.IsZero() {
		t.Errorf("cold feed: %+v", cold)
	}
}

func TestFetch_CapsBody(t *testing.T) {
	prev := maxFeedBytes
	maxFeedBytes = int64(len(rssBody))
	t.Cleanup(func() { maxFeedBytes = prev })

	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(rssBody + " "))
	}))
	t.Cleanup(big.Close)
	f, db := newFetcher(t)
	if err := f.Fetch(big.URL).Err; err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("want a too-large error, got %v", err)
	}
	if _, ok, _ := db.LoadFeedCache(big.URL); ok {
		t.Error("an oversized feed must not be cached")
	}

	exact := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(rssBody))
	}))
	t.Cleanup(exact.Close)
	if feed := f.Fetch(exact.URL); feed.Err != nil || len(feed.Posts) != 1 {
		t.Errorf("a feed at the cap should load: %v", feed.Err)
	}
}
