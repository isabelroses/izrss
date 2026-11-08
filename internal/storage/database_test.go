package storage

import (
	"bytes"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNew(t *testing.T) {
	if db := setupTestDB(t); db.conn == nil {
		t.Error("Expected non-nil connection")
	}
}

func TestNew_InvalidPath(t *testing.T) {
	if _, err := New("/nonexistent/path/to/db.db"); err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestClose(t *testing.T) {
	db := setupTestDB(t)
	if err := db.Close(); err != nil {
		t.Errorf("Unexpected error closing database: %v", err)
	}
	db.conn = nil
	if err := db.Close(); err != nil {
		t.Errorf("Unexpected error closing nil connection: %v", err)
	}
}

func TestSaveAndLoadPostReadStatus(t *testing.T) {
	db := setupTestDB(t)
	must(t, db.SavePostReadStatus("http://example.com/feed", "uuid-1", true))
	statuses, err := db.LoadPostReadStatuses()
	must(t, err)
	if len(statuses) != 1 || !statuses[ReadKey{"http://example.com/feed", "uuid-1"}] {
		t.Errorf("Expected only uuid-1, read; got %v", statuses)
	}
}

func TestSavePostReadStatus_Update(t *testing.T) {
	db := setupTestDB(t)
	must(t, db.SavePostReadStatus("http://example.com/feed", "uuid-1", true))
	must(t, db.SavePostReadStatus("http://example.com/feed", "uuid-1", false))
	statuses, err := db.LoadPostReadStatuses()
	must(t, err)
	if read, ok := statuses[ReadKey{"http://example.com/feed", "uuid-1"}]; !ok || read {
		t.Errorf("Expected uuid-1 to be unread after update, got %v", statuses)
	}
}

func TestSavePostReadStatuses_Batch(t *testing.T) {
	db := setupTestDB(t)
	must(t, db.SavePostReadStatuses([]PostReadStatus{
		{FeedURL: "http://example.com/feed1", Key: "uuid-1", Read: true},
		{FeedURL: "http://example.com/feed1", Key: "uuid-2", Read: false},
		{FeedURL: "http://example.com/feed2", Key: "uuid-3", Read: true},
	}))
	loaded, err := db.LoadPostReadStatuses()
	must(t, err)
	if len(loaded) != 3 || !loaded[ReadKey{"http://example.com/feed1", "uuid-1"}] ||
		loaded[ReadKey{"http://example.com/feed1", "uuid-2"}] || !loaded[ReadKey{"http://example.com/feed2", "uuid-3"}] {
		t.Errorf("Expected uuid-1 and uuid-3 read, uuid-2 unread; got %v", loaded)
	}
}

func TestSavePostReadStatus_SameKeyInTwoFeeds(t *testing.T) {
	db := setupTestDB(t)
	must(t, db.SavePostReadStatus("a", "shared", true))
	must(t, db.SavePostReadStatus("b", "shared", false))
	loaded, err := db.LoadPostReadStatuses()
	must(t, err)
	if !loaded[ReadKey{"a", "shared"}] || loaded[ReadKey{"b", "shared"}] {
		t.Errorf("feeds should be tracked apart, got %v", loaded)
	}
}

// oldSchemaDB writes a database in the pre-migration layout, keyed by uuid
// alone, and returns its path.
func oldSchemaDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "old.db")
	conn, err := sql.Open("sqlite3", path)
	must(t, err)
	defer func() { _ = conn.Close() }()
	_, err = conn.Exec(`
		CREATE TABLE post_read_status (
			uuid TEXT PRIMARY KEY,
			feed_url TEXT NOT NULL,
			read INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE cache_metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		CREATE TABLE feed_cache (url TEXT PRIMARY KEY, content BLOB NOT NULL, fetched_at TEXT NOT NULL);
		CREATE INDEX idx_feed_url ON post_read_status(feed_url);
		INSERT INTO post_read_status VALUES ('g1', 'https://a/feed', 1), ('g2', 'https://a/feed', 0),
			('g3', 'https://b/feed', 1), ('', 'https://b/feed', 1);
		INSERT INTO feed_cache VALUES ('https://a/feed', x'3c7273733e', '2026-01-01T00:00:00Z');
	`)
	must(t, err)
	return path
}

func TestMigration_PreservesReadFlags(t *testing.T) {
	path := oldSchemaDB(t)
	for range 2 { // a second open must be a no-op
		db, err := New(path)
		must(t, err)
		loaded, err := db.LoadPostReadStatuses()
		must(t, err)
		want := map[ReadKey]bool{
			{"https://a/feed", "g1"}: true,
			{"https://a/feed", "g2"}: false,
			{"https://b/feed", "g3"}: true,
		}
		if len(loaded) != len(want) {
			t.Errorf("got %v, want %v (the GUID-less row dropped)", loaded, want)
		}
		for k, v := range want {
			if r, ok := loaded[k]; !ok || r != v {
				t.Errorf("%v: got %v (present %v), want %v", k, r, ok, v)
			}
		}
		if c, err := db.LoadFeedCache("https://a/feed"); err != nil || string(c) != "<rss>" {
			t.Errorf("feed cache lost: %q %v", c, err)
		}
		must(t, db.Close())
	}
	db, err := New(path)
	must(t, err)
	defer func() { _ = db.Close() }()
	must(t, db.SavePostReadStatus("https://c/feed", "g1", true)) // the new key works
	loaded, err := db.LoadPostReadStatuses()
	must(t, err)
	if !loaded[ReadKey{"https://c/feed", "g1"}] || !loaded[ReadKey{"https://a/feed", "g1"}] {
		t.Errorf("g1 should be read in both feeds: %v", loaded)
	}
}

func TestMigration_FreshDBIsCurrent(t *testing.T) {
	db := setupTestDB(t)
	var v int
	must(t, db.conn.QueryRow(`PRAGMA user_version`).Scan(&v))
	if v != schemaVersion {
		t.Errorf("user_version %d, want %d", v, schemaVersion)
	}
}

func TestSavePostReadStatuses_Empty(t *testing.T) {
	if err := setupTestDB(t).SavePostReadStatuses([]PostReadStatus{}); err != nil {
		t.Errorf("Unexpected error saving empty statuses: %v", err)
	}
}

func TestLoadPostReadStatuses_Empty(t *testing.T) {
	statuses, err := setupTestDB(t).LoadPostReadStatuses()
	must(t, err)
	if len(statuses) != 0 {
		t.Errorf("Expected 0 statuses, got %d", len(statuses))
	}
}

func TestGetCacheTime_NotSet(t *testing.T) {
	cacheTime, err := setupTestDB(t).GetCacheTime()
	must(t, err)
	if cacheTime != nil {
		t.Error("Expected nil cache time when not set")
	}
}

func TestSetAndGetCacheTime(t *testing.T) {
	db := setupTestDB(t)
	before := time.Now().Add(-time.Second)
	must(t, db.SetCacheTime())
	after := time.Now().Add(time.Second)

	cacheTime, err := db.GetCacheTime()
	must(t, err)
	if cacheTime == nil {
		t.Fatal("Expected non-nil cache time")
	}
	if cacheTime.Before(before) || cacheTime.After(after) {
		t.Errorf("Cache time %v not in expected range [%v, %v]", cacheTime, before, after)
	}
}

func TestSetCacheTime_Update(t *testing.T) {
	db := setupTestDB(t)
	get := func() time.Time {
		t.Helper()
		must(t, db.SetCacheTime())
		at, err := db.GetCacheTime()
		must(t, err)
		if at == nil {
			t.Fatal("Expected non-nil cache time after set")
		}
		return *at
	}
	first, second := get(), get()
	if second.Before(first) {
		t.Errorf("Expected second cache time %v to not be before first %v", second, first)
	}
}

func TestSaveFeedCache(t *testing.T) {
	const url = "http://example.com/feed.xml"
	tests := []struct {
		name  string
		saves [][]byte // saved in order; the last should be loaded
	}{
		{"round trip", [][]byte{[]byte("<rss><channel><title>Test</title></channel></rss>")}},
		{"update", [][]byte{[]byte("<rss>version1</rss>"), []byte("<rss>version2</rss>")}},
		{"binary", [][]byte{{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x89, 0x50, 0x4E, 0x47}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			for _, content := range tt.saves {
				must(t, db.SaveFeedCache(url, content))
			}
			loaded, err := db.LoadFeedCache(url)
			must(t, err)
			if want := tt.saves[len(tt.saves)-1]; !bytes.Equal(loaded, want) {
				t.Errorf("Expected %q, got %q", want, loaded)
			}
		})
	}
}

func TestLoadFeedCache_NotFound(t *testing.T) {
	content, err := setupTestDB(t).LoadFeedCache("http://nonexistent.com/feed.xml")
	must(t, err)
	if content != nil {
		t.Error("Expected nil content for non-existent cache")
	}
}
