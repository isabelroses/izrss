package readstate

import (
	"testing"

	"github.com/isabelroses/izrss/internal/rss"
	"github.com/isabelroses/izrss/internal/storage"
)

func testDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.New(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// reopened is a State that has only what db holds, as after a restart.
func reopened(t *testing.T, db *storage.DB) *State {
	t.Helper()
	s := New(db)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	return s
}

func feed(url string, posts ...rss.Post) rss.Feed { return rss.Feed{URL: url, Posts: posts} }

func TestSetSurvivesRestart(t *testing.T) {
	db := testDB(t)
	s := New(db)
	ref := rss.PostRef{FeedURL: "a", Key: "1"}
	if err := s.Set(ref, true); err != nil {
		t.Fatal(err)
	}
	if !reopened(t, db).IsRead("a", "1") {
		t.Error("a read mark should be saved")
	}
	if err := s.Set(ref, false); err != nil {
		t.Fatal(err)
	}
	if reopened(t, db).IsRead("a", "1") || s.IsRead("a", "1") {
		t.Error("an unread mark should be saved")
	}
}

func TestGUIDLessPostsAreIndependent(t *testing.T) {
	db := testDB(t)
	a := feed("a", rss.Post{Link: "https://a/1", Title: "one"}, rss.Post{Link: "https://a/2", Title: "two"}, rss.Post{Title: "t", Date: "d"})
	s := New(db)
	if err := s.Set(a.Posts[0].Ref("a"), true); err != nil {
		t.Fatal(err)
	}
	got := reopened(t, db)
	if got.Unread(a) != 2 || got.IsRead("a", a.Posts[1].Key()) {
		t.Errorf("only post one should be read: unread %d", got.Unread(a))
	}
}

func TestSameGUIDInTwoFeedsIsIndependent(t *testing.T) {
	db := testDB(t)
	a := feed("a", rss.Post{UUID: "shared"})
	b := feed("b", rss.Post{UUID: "shared"})
	s := New(db)
	if err := s.Set(rss.PostRef{FeedURL: "b", Key: "shared"}, true); err != nil {
		t.Fatal(err)
	}
	got := reopened(t, db)
	if got.Unread(a) != 1 || got.Unread(b) != 0 {
		t.Errorf("unread a %d b %d, want 1, 0", got.Unread(a), got.Unread(b))
	}
}

// rowCount is how many read rows db holds.
func rowCount(t *testing.T, db *storage.DB) int {
	t.Helper()
	saved, err := db.LoadPostReadStatuses()
	if err != nil {
		t.Fatal(err)
	}
	return len(saved)
}

func TestOnlyChangedRowsAreWritten(t *testing.T) {
	db := testDB(t)
	s := New(db)
	a := feed("a", rss.Post{UUID: "1"}, rss.Post{UUID: "2"}, rss.Post{UUID: "3"})
	b := feed("b", rss.Post{UUID: "1"}, rss.Post{UUID: "2"})

	if err := s.Set(a.Posts[0].Ref("a"), true); err != nil {
		t.Fatal(err)
	}
	// A repeated mark must not write: with the database closed, a write fails.
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(a.Posts[0].Ref("a"), true); err != nil {
		t.Errorf("a repeated mark wrote: %v", err)
	}
	db = testDB(t)
	s = New(db)
	if err := s.MarkFeedRead(a); err != nil {
		t.Fatal(err)
	}
	if n := rowCount(t, db); n != 3 {
		t.Errorf("marking a feed read wrote %d rows, want its 3 posts only (nothing of feed b)", n)
	}
	if s.Unread(a) != 0 || s.Unread(b) != 2 {
		t.Errorf("unread a %d b %d, want 0, 2", s.Unread(a), s.Unread(b))
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFeedRead(a); err != nil {
		t.Errorf("marking a read feed wrote: %v", err)
	}
}

// A refresh swaps a feed's posts for new ones; what is known about a post
// carries over by identity, and new posts are unread.
func TestKnowledgeOutlivesFeedReplacement(t *testing.T) {
	s := New(testDB(t))
	if err := s.Set(rss.PostRef{FeedURL: "a", Key: "1"}, true); err != nil {
		t.Fatal(err)
	}
	refreshed := feed("a", rss.Post{UUID: "1"}, rss.Post{UUID: "2"})
	if s.Unread(refreshed) != 1 {
		t.Errorf("unread = %d, want 1 (post 2 is new)", s.Unread(refreshed))
	}
}

func TestSnapshotRestoreKeepsLaterMarks(t *testing.T) {
	db := testDB(t)
	if err := New(db).Set(rss.PostRef{FeedURL: "a", Key: "old"}, true); err != nil {
		t.Fatal(err)
	}
	saved, err := db.LoadPostReadStatuses()
	if err != nil {
		t.Fatal(err)
	}
	s := New(db)
	if err := s.Set(rss.PostRef{FeedURL: "a", Key: "new"}, true); err != nil {
		t.Fatal(err)
	}
	s.Restore(saved)
	if !s.IsRead("a", "old") || !s.IsRead("a", "new") {
		t.Error("restoring should add the saved marks and keep newer ones")
	}
}
