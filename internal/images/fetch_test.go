package images

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func server(t *testing.T, contentType string, body []byte, delay time.Duration) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(delay)
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestLoad_FetchesAndCaches(t *testing.T) {
	body := []byte("\x89PNG fake")
	srv, hits := server(t, "image/png", body, 0)
	dir := t.TempDir()
	f := NewFetcher(dir, 1<<20, time.Second)
	url := srv.URL + "/a.png"

	for range 2 {
		got, err := f.Load(url)
		if err != nil || string(got) != string(body) {
			t.Fatalf("Load = %q, %v", got, err)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("second load should come from cache, server hit %d times", hits.Load())
	}
	sum := sha256.Sum256([]byte(url))
	if _, err := os.Stat(filepath.Join(dir, hex.EncodeToString(sum[:]))); err != nil {
		t.Errorf("cache file missing: %v", err)
	}
}

func TestLoad_RejectsNonImage(t *testing.T) {
	srv, _ := server(t, "text/html", []byte("<html>"), 0)
	dir := t.TempDir()
	if _, err := NewFetcher(dir, 1<<20, time.Second).Load(srv.URL); !errors.Is(err, ErrNotImage) {
		t.Errorf("err = %v, want ErrNotImage", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("nothing should be cached, found %d files", len(entries))
	}
}

func TestLoad_RejectsOversize(t *testing.T) {
	srv, _ := server(t, "image/png", []byte(strings.Repeat("x", 101)), 0)
	dir := t.TempDir()
	if _, err := NewFetcher(dir, 100, time.Second).Load(srv.URL); !errors.Is(err, ErrTooLarge) {
		t.Errorf("err = %v, want ErrTooLarge", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("nothing should be cached, found %d files", len(entries))
	}
}

func TestLoad_RejectsScheme(t *testing.T) {
	f := NewFetcher(t.TempDir(), 1<<20, time.Second)
	for _, u := range []string{"file:///etc/passwd", "ftp://example.com/a.png", "/relative.png"} {
		if _, err := f.Load(u); !errors.Is(err, ErrScheme) {
			t.Errorf("Load(%q) err = %v, want ErrScheme", u, err)
		}
	}
}

func TestLoad_Timeout(t *testing.T) {
	srv, _ := server(t, "image/png", []byte("x"), 300*time.Millisecond)
	if _, err := NewFetcher(t.TempDir(), 1<<20, 50*time.Millisecond).Load(srv.URL); err == nil {
		t.Error("expected a timeout error")
	}
}

func TestLoad_CacheUnwritable(t *testing.T) {
	srv, _ := server(t, "image/png", []byte("img"), 0)
	notDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := NewFetcher(notDir, 1<<20, time.Second).Load(srv.URL)
	if err != nil || string(got) != "img" {
		t.Errorf("should fall back to HTTP: %q, %v", got, err)
	}
}

func TestLoad_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	if _, err := NewFetcher(t.TempDir(), 1<<20, time.Second).Load(srv.URL); err == nil {
		t.Error("404 should be an error")
	}
}
