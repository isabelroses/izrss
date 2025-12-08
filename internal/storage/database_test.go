package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "izrss-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := New(dbPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create database: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestNew(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	if db == nil {
		t.Fatal("Expected non-nil database")
	}
	if db.conn == nil {
		t.Error("Expected non-nil connection")
	}
}

func TestNew_InvalidPath(t *testing.T) {
	_, err := New("/nonexistent/path/to/db.db")
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestClose(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := db.Close()
	if err != nil {
		t.Errorf("Unexpected error closing database: %v", err)
	}

	// Closing nil connection should not error
	db.conn = nil
	err = db.Close()
	if err != nil {
		t.Errorf("Unexpected error closing nil connection: %v", err)
	}
}

func TestSaveAndLoadPostReadStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Save a single post status
	err := db.SavePostReadStatus("uuid-1", "http://example.com/feed", true)
	if err != nil {
		t.Fatalf("Failed to save post read status: %v", err)
	}

	// Load and verify
	statuses, err := db.LoadPostReadStatuses()
	if err != nil {
		t.Fatalf("Failed to load post read statuses: %v", err)
	}

	if len(statuses) != 1 {
		t.Errorf("Expected 1 status, got %d", len(statuses))
	}

	if !statuses["uuid-1"] {
		t.Error("Expected uuid-1 to be read")
	}
}

func TestSavePostReadStatus_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Save as read
	err := db.SavePostReadStatus("uuid-1", "http://example.com/feed", true)
	if err != nil {
		t.Fatalf("Failed to save post read status: %v", err)
	}

	// Update to unread
	err = db.SavePostReadStatus("uuid-1", "http://example.com/feed", false)
	if err != nil {
		t.Fatalf("Failed to update post read status: %v", err)
	}

	statuses, err := db.LoadPostReadStatuses()
	if err != nil {
		t.Fatalf("Failed to load post read statuses: %v", err)
	}

	if statuses["uuid-1"] {
		t.Error("Expected uuid-1 to be unread after update")
	}
}

func TestSavePostReadStatuses_Batch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	statuses := []PostReadStatus{
		{UUID: "uuid-1", FeedURL: "http://example.com/feed1", Read: true},
		{UUID: "uuid-2", FeedURL: "http://example.com/feed1", Read: false},
		{UUID: "uuid-3", FeedURL: "http://example.com/feed2", Read: true},
	}

	err := db.SavePostReadStatuses(statuses)
	if err != nil {
		t.Fatalf("Failed to save batch post read statuses: %v", err)
	}

	loaded, err := db.LoadPostReadStatuses()
	if err != nil {
		t.Fatalf("Failed to load post read statuses: %v", err)
	}

	if len(loaded) != 3 {
		t.Errorf("Expected 3 statuses, got %d", len(loaded))
	}

	if !loaded["uuid-1"] {
		t.Error("Expected uuid-1 to be read")
	}
	if loaded["uuid-2"] {
		t.Error("Expected uuid-2 to be unread")
	}
	if !loaded["uuid-3"] {
		t.Error("Expected uuid-3 to be read")
	}
}

func TestSavePostReadStatuses_Empty(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := db.SavePostReadStatuses([]PostReadStatus{})
	if err != nil {
		t.Errorf("Unexpected error saving empty statuses: %v", err)
	}
}

func TestLoadPostReadStatuses_Empty(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	statuses, err := db.LoadPostReadStatuses()
	if err != nil {
		t.Fatalf("Failed to load empty post read statuses: %v", err)
	}

	if len(statuses) != 0 {
		t.Errorf("Expected 0 statuses, got %d", len(statuses))
	}
}

func TestGetCacheTime_NotSet(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cacheTime, err := db.GetCacheTime()
	if err != nil {
		t.Fatalf("Unexpected error getting cache time: %v", err)
	}

	if cacheTime != nil {
		t.Error("Expected nil cache time when not set")
	}
}

func TestSetAndGetCacheTime(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	before := time.Now().Add(-time.Second)

	err := db.SetCacheTime()
	if err != nil {
		t.Fatalf("Failed to set cache time: %v", err)
	}

	after := time.Now().Add(time.Second)

	cacheTime, err := db.GetCacheTime()
	if err != nil {
		t.Fatalf("Failed to get cache time: %v", err)
	}

	if cacheTime == nil {
		t.Fatal("Expected non-nil cache time")
	}

	if cacheTime.Before(before) || cacheTime.After(after) {
		t.Errorf("Cache time %v not in expected range [%v, %v]", cacheTime, before, after)
	}
}

func TestSetCacheTime_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Set first time
	err := db.SetCacheTime()
	if err != nil {
		t.Fatalf("Failed to set cache time: %v", err)
	}

	first, err := db.GetCacheTime()
	if err != nil {
		t.Fatalf("Failed to get first cache time: %v", err)
	}

	if first == nil {
		t.Fatal("Expected non-nil cache time after first set")
	}

	// Set again (update)
	err = db.SetCacheTime()
	if err != nil {
		t.Fatalf("Failed to update cache time: %v", err)
	}

	second, err := db.GetCacheTime()
	if err != nil {
		t.Fatalf("Failed to get second cache time: %v", err)
	}

	if second == nil {
		t.Fatal("Expected non-nil cache time after second set")
	}

	// Second should be at least equal to first (time only advances)
	if second.Before(*first) {
		t.Errorf("Expected second cache time %v to not be before first %v", second, first)
	}
}

func TestSaveAndLoadFeedCache(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	url := "http://example.com/feed.xml"
	content := []byte("<rss><channel><title>Test</title></channel></rss>")

	err := db.SaveFeedCache(url, content)
	if err != nil {
		t.Fatalf("Failed to save feed cache: %v", err)
	}

	loaded, err := db.LoadFeedCache(url)
	if err != nil {
		t.Fatalf("Failed to load feed cache: %v", err)
	}

	if string(loaded) != string(content) {
		t.Errorf("Expected %q, got %q", string(content), string(loaded))
	}
}

func TestLoadFeedCache_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	content, err := db.LoadFeedCache("http://nonexistent.com/feed.xml")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if content != nil {
		t.Error("Expected nil content for non-existent cache")
	}
}

func TestSaveFeedCache_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	url := "http://example.com/feed.xml"
	content1 := []byte("<rss>version1</rss>")
	content2 := []byte("<rss>version2</rss>")

	err := db.SaveFeedCache(url, content1)
	if err != nil {
		t.Fatalf("Failed to save first feed cache: %v", err)
	}

	err = db.SaveFeedCache(url, content2)
	if err != nil {
		t.Fatalf("Failed to save second feed cache: %v", err)
	}

	loaded, err := db.LoadFeedCache(url)
	if err != nil {
		t.Fatalf("Failed to load feed cache: %v", err)
	}

	if string(loaded) != string(content2) {
		t.Errorf("Expected updated content %q, got %q", string(content2), string(loaded))
	}
}

func TestClearFeedCache(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Save multiple feeds
	err := db.SaveFeedCache("http://example1.com/feed.xml", []byte("content1"))
	if err != nil {
		t.Fatalf("Failed to save first feed: %v", err)
	}

	err = db.SaveFeedCache("http://example2.com/feed.xml", []byte("content2"))
	if err != nil {
		t.Fatalf("Failed to save second feed: %v", err)
	}

	// Clear all
	err = db.ClearFeedCache()
	if err != nil {
		t.Fatalf("Failed to clear feed cache: %v", err)
	}

	// Verify both are gone
	content1, err := db.LoadFeedCache("http://example1.com/feed.xml")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if content1 != nil {
		t.Error("Expected nil content after clear")
	}

	content2, err := db.LoadFeedCache("http://example2.com/feed.xml")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if content2 != nil {
		t.Error("Expected nil content after clear")
	}
}

func TestSaveFeedCache_BinaryContent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	url := "http://example.com/feed.xml"
	// Binary content with null bytes and special characters
	content := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x89, 0x50, 0x4E, 0x47}

	err := db.SaveFeedCache(url, content)
	if err != nil {
		t.Fatalf("Failed to save binary feed cache: %v", err)
	}

	loaded, err := db.LoadFeedCache(url)
	if err != nil {
		t.Fatalf("Failed to load binary feed cache: %v", err)
	}

	if len(loaded) != len(content) {
		t.Errorf("Expected %d bytes, got %d", len(content), len(loaded))
	}

	for i := range content {
		if loaded[i] != content[i] {
			t.Errorf("Byte %d: expected %x, got %x", i, content[i], loaded[i])
		}
	}
}
