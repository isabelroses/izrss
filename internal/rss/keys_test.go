package rss

import (
	"bytes"
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
		{Post{Title: "t", Date: "d"}, "t\x00d"},
	}
	for _, tt := range tests {
		if got := tt.post.Key(); got != tt.want {
			t.Errorf("Key(%+v) = %q, want %q", tt.post, got, tt.want)
		}
	}
}

func twoFeeds() Feeds {
	return Feeds{
		{URL: "a", Posts: []Post{{UUID: "a1"}, {UUID: "shared"}}},
		{URL: "b", Posts: []Post{{UUID: "b1"}, {UUID: "shared"}}},
	}
}

func TestLocate(t *testing.T) {
	fs := twoFeeds()
	if loc, ok := fs.Locate("", "b1"); !ok || loc != (Loc{Feed: 1, Post: 0}) {
		t.Errorf("Locate any b1 = %+v, %v", loc, ok)
	}
	if loc, ok := fs.Locate("a", "a1"); !ok || loc != (Loc{Feed: 0, Post: 0}) {
		t.Errorf("Locate a/a1 = %+v, %v", loc, ok)
	}
	if _, ok := fs.Locate("", "missing"); ok {
		t.Error("missing key found")
	}
}

func TestLocate_DuplicateKeysPreferFeed(t *testing.T) {
	if loc, ok := twoFeeds().Locate("b", "shared"); !ok || loc != (Loc{Feed: 1, Post: 1}) {
		t.Errorf("should prefer feed b's copy, got %+v", loc)
	}
}

func TestSetReadAt(t *testing.T) {
	fs := twoFeeds()
	fs.SetRead(Loc{Feed: 1, Post: 1}, true)
	if !fs[1].Posts[1].Read || fs[0].Posts[1].Read {
		t.Errorf("only feed b's shared post should be read: %+v", fs)
	}
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

func TestGetContentForURL_ErrorFields(t *testing.T) {
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prev) })

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(bad.Close)
	feed := NewFetcher(testDB(t), "02/01/2006").GetContentForURL(bad.URL, false)
	if feed.Err == nil {
		t.Error("a 500 should set Err")
	}
	if !strings.Contains(feed.Title, strings.TrimPrefix(bad.URL, "http://")) {
		t.Errorf("title should name the feed host, got %q", feed.Title)
	}

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(rssBody))
	}))
	t.Cleanup(good.Close)
	db := testDB(t)
	f := NewFetcher(db, "02/01/2006")
	_ = db.Close() // cache writes now fail
	feed = f.GetContentForURL(good.URL, false)
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
	f := NewFetcher(testDB(t), "02/01/2006")
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			f.GetContentForURL(srv.URL+"/"+string(rune('a'+i)), false)
		})
	}
	wg.Wait()
	if p := peak.Load(); p > maxConcurrentFetches {
		t.Errorf("peak in-flight fetches %d > %d", p, maxConcurrentFetches)
	}
}

// Titles reach the TUI's panes and the reader header verbatim; control
// characters in them would be interpreted by the terminal.
func TestGetContentForURL_SanitizesTitles(t *testing.T) {
	// XML forbids these characters, but JSON Feed (also parsed) allows them.
	body := `{"version":"https://jsonfeed.org/version/1.1","title":"Evil\u001b]0;pwned\u0007 feed",
"items":[{"id":"g","title":"clear\u001b[2J me","content_text":"d"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	feed := NewFetcher(testDB(t), "02/01/2006").GetContentForURL(srv.URL, false)
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

// reloaded returns fresh copies of feeds with read state loaded from db.
func reloaded(t *testing.T, db *storage.DB, feeds Feeds) Feeds {
	t.Helper()
	fresh := make(Feeds, len(feeds))
	for i, f := range feeds {
		fresh[i] = f
		fresh[i].Posts = make([]Post, len(f.Posts))
		for j, p := range f.Posts {
			p.Read = false
			fresh[i].Posts[j] = p
		}
	}
	if err := fresh.ReadTracking(db); err != nil {
		t.Fatal(err)
	}
	return fresh
}

func TestTracking_GUIDLessPostsAreIndependent(t *testing.T) {
	db := testDB(t)
	feeds := Feeds{{URL: "a", Posts: []Post{
		{Link: "https://a/1", Title: "one"},
		{Link: "https://a/2", Title: "two"},
	}}}
	feeds.SetRead(Loc{0, 0}, true)
	if err := feeds.WritePostTrackingAt(db, Loc{0, 0}); err != nil {
		t.Fatal(err)
	}
	got := reloaded(t, db, feeds)
	if !got[0].Posts[0].Read || got[0].Posts[1].Read {
		t.Errorf("only post one should be read: %v %v", got[0].Posts[0].Read, got[0].Posts[1].Read)
	}

	// The bulk write must keep them apart too.
	if err := got.WriteTracking(db); err != nil {
		t.Fatal(err)
	}
	got = reloaded(t, db, feeds)
	if !got[0].Posts[0].Read || got[0].Posts[1].Read {
		t.Errorf("after WriteTracking: %v %v", got[0].Posts[0].Read, got[0].Posts[1].Read)
	}
}

func TestTracking_SameGUIDInTwoFeedsIsIndependent(t *testing.T) {
	db := testDB(t)
	feeds := twoFeeds()
	feeds.SetRead(Loc{1, 1}, true) // "shared" in feed b only
	if err := feeds.WriteTracking(db); err != nil {
		t.Fatal(err)
	}
	got := reloaded(t, db, feeds)
	if got[0].Posts[1].Read || !got[1].Posts[1].Read {
		t.Errorf("shared: feed a read %v, feed b read %v; want false, true", got[0].Posts[1].Read, got[1].Posts[1].Read)
	}
}

func TestGetCachedContent(t *testing.T) {
	db := testDB(t)
	if err := db.SaveFeedCache("https://cached.example/feed", []byte(rssBody)); err != nil {
		t.Fatal(err)
	}
	feeds := NewFetcher(db, "02/01/2006").GetCachedContent([]string{"https://cached.example/feed", "https://cold.example/feed"})
	if feeds[0].Title != "T" || len(feeds[0].Posts) != 1 || feeds[0].Err != nil {
		t.Errorf("cached feed: %+v", feeds[0])
	}
	if feeds[1].Title != "cold.example" || len(feeds[1].Posts) != 0 || feeds[1].Err != nil {
		t.Errorf("cold feed: %+v", feeds[1])
	}
}

func TestFetchURL_CapsBody(t *testing.T) {
	prev := maxFeedBytes
	maxFeedBytes = int64(len(rssBody))
	t.Cleanup(func() { maxFeedBytes = prev })

	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(rssBody + " "))
	}))
	t.Cleanup(big.Close)
	db := testDB(t)
	f := NewFetcher(db, "02/01/2006")
	if _, _, err := f.FetchURL(big.URL, false); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("want a too-large error, got %v", err)
	}
	if c, _ := db.LoadFeedCache(big.URL); c != nil {
		t.Error("an oversized feed must not be cached")
	}

	exact := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(rssBody))
	}))
	t.Cleanup(exact.Close)
	if feed := f.GetContentForURL(exact.URL, false); feed.Err != nil || len(feed.Posts) != 1 {
		t.Errorf("a feed at the cap should load: %v", feed.Err)
	}
}

// ReadTrackingAt loads one feed's read state exactly as ReadTracking does,
// leaving the other feeds alone.
func TestReadTrackingAt_MatchesReadTracking(t *testing.T) {
	db := testDB(t)
	feeds := Feeds{
		{URL: "a", Posts: []Post{{UUID: "shared"}, {Link: "https://a/1"}, {Title: "t", Date: "d"}}},
		{URL: "b", Posts: []Post{{UUID: "shared"}, {Link: "https://a/1"}}},
	}
	feeds.SetRead(Loc{0, 0}, true)
	feeds.SetRead(Loc{0, 2}, true)
	feeds.SetRead(Loc{1, 1}, true)
	if err := feeds.WriteTracking(db); err != nil {
		t.Fatal(err)
	}
	want := reloaded(t, db, feeds)
	for i := range feeds {
		got := reloaded(t, db, feeds)
		for k := range got {
			for j := range got[k].Posts {
				got[k].Posts[j].Read = false
			}
		}
		if err := got.ReadTrackingAt(db, i); err != nil {
			t.Fatal(err)
		}
		for k := range got {
			for j, p := range got[k].Posts {
				if w := k == i && want[k].Posts[j].Read; p.Read != w {
					t.Errorf("feed %d: feeds[%d].Posts[%d].Read = %v, want %v", i, k, j, p.Read, w)
				}
			}
		}
	}
}
