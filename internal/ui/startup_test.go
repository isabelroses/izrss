package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/rss"
)

// The startup load paints from cache alone; feeds missing from the cache
// wait for the refresh rather than blocking first paint on the network.
func TestLoadCachedFeeds_NoNetwork(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`<rss version="2.0"><channel><title>T</title></channel></rss>`))
	}))
	t.Cleanup(srv.Close)

	cfg := config.Default()
	cfg.Urls = []string{srv.URL + "/a", srv.URL + "/b"}
	db := testDB(t)
	m := NewModel(cfg, db, rss.NewFetcher(db, cfg.DateFormat), nil, nil)

	msg, ok := m.loadCachedFeeds()().(feedsRefreshedMsg)
	if !ok {
		t.Fatal("want feedsRefreshedMsg")
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("startup load made %d requests, want 0", n)
	}
	if len(msg.feeds) != 2 {
		t.Fatalf("want a placeholder per url, got %d feeds", len(msg.feeds))
	}
	for i, f := range msg.feeds {
		if f.URL != cfg.Urls[i] || f.Err != nil || len(f.Posts) != 0 {
			t.Errorf("feed %d: url %q err %v posts %d", i, f.URL, f.Err, len(f.Posts))
		}
		if host := strings.TrimPrefix(srv.URL, "http://"); f.Name() != host || f.Title != "" {
			t.Errorf("feed %d title %q name %q, want name %q", i, f.Title, f.Name(), host)
		}
	}
	if len(msg.errs) != 0 {
		t.Errorf("a cold cache is not an error: %v", msg.errs)
	}
}
