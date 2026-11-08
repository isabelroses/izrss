package library

import (
	"slices"
	"testing"
	"time"

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

func feed(url string, posts ...rss.Post) rss.Feed { return rss.Feed{URL: url, Posts: posts} }

// open is a library over db showing feeds, as after a start with them cached.
func open(t *testing.T, db *storage.DB, feeds ...rss.Feed) *Library {
	t.Helper()
	l := New(db, rss.NewStore(db))
	snap := l.Load(nil)
	if snap.MarksErr != nil {
		t.Fatal(snap.MarksErr)
	}
	snap.Feeds = feeds
	l.Restore(snap)
	return l
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestVersionMovesOnlyOnChange(t *testing.T) {
	l := open(t, testDB(t), feed("a", rss.Post{UUID: "1"}, rss.Post{UUID: "2"}))
	ref := rss.PostRef{FeedURL: "a", Key: "1"}
	steps := []struct {
		name    string
		do      func() error
		changes bool
	}{
		{"mark read", func() error { return l.SetRead(ref, true) }, true},
		{"mark read again", func() error { return l.SetRead(ref, true) }, false},
		{"unknown post", func() error { return l.SetRead(rss.PostRef{FeedURL: "a", Key: "x"}, true) }, false},
		{"mark feed read", func() error { return l.MarkRead([]string{"a"}) }, true},
		{"replace feed", func() error { l.Replace(rss.Feed{URL: "a"}); return nil }, true},
	}
	for _, s := range steps {
		before := l.Version()
		if err := s.do(); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if moved := l.Version() != before; moved != s.changes {
			t.Errorf("%s: version moved %v, want %v", s.name, moved, s.changes)
		}
	}
}

// Loading brings back the cached feeds, the saved marks and the latest
// fetch time; marks made after the load was read win.
func TestLoadRestoresFeedsAndMarks(t *testing.T) {
	db := testDB(t)
	store := rss.NewStore(db)
	a := feed("https://a.example/feed", rss.Post{UUID: "old"}, rss.Post{UUID: "new"})
	must(t, store.Save(a))
	l := open(t, db, a)
	must(t, l.SetRead(rss.PostRef{FeedURL: a.URL, Key: "old"}, true))

	l = New(db, store)
	snap := l.Load([]string{a.URL, "https://cold.example/feed"})
	if snap.MarksErr != nil || len(snap.Feeds) != 2 || snap.Fetched().IsZero() {
		t.Fatalf("snapshot: %+v, fetched %v", snap, snap.Fetched())
	}
	l.Restore(snap) // as if the reload raced a new mark...
	must(t, l.SetRead(rss.PostRef{FeedURL: a.URL, Key: "new"}, true))
	l.Restore(snap) // ...which the older snapshot must not undo
	if !l.IsRead(rss.PostRef{FeedURL: a.URL, Key: "old"}) || !l.IsRead(rss.PostRef{FeedURL: a.URL, Key: "new"}) {
		t.Error("restoring should add the saved marks and keep newer ones")
	}
	if l.Len() != 2 || l.Unread() != 0 {
		t.Errorf("%d feeds, %d unread; want 2, 0", l.Len(), l.Unread())
	}
}

func TestSetReadSurvivesRestart(t *testing.T) {
	db := testDB(t)
	a := feed("a", rss.Post{UUID: "1"})
	l := open(t, db, a)
	ref := rss.PostRef{FeedURL: "a", Key: "1"}
	must(t, l.SetRead(ref, true))
	if !open(t, db, a).IsRead(ref) {
		t.Error("a read mark should be saved")
	}
	must(t, l.SetRead(ref, false))
	if open(t, db, a).IsRead(ref) || l.IsRead(ref) {
		t.Error("an unread mark should be saved")
	}
}

func TestGUIDLessPostsAreIndependent(t *testing.T) {
	db := testDB(t)
	a := feed("a", rss.Post{Link: "https://a/1", Title: "one"}, rss.Post{Link: "https://a/2", Title: "two"}, rss.Post{Title: "t", RawDate: "d"})
	must(t, open(t, db, a).SetRead(a.Posts[0].Ref("a"), true))
	got := open(t, db, a)
	if got.Unread() != 2 || got.IsRead(a.Posts[1].Ref("a")) {
		t.Errorf("only post one should be read: unread %d", got.Unread())
	}
}

func TestSameGUIDInTwoFeedsIsIndependent(t *testing.T) {
	db := testDB(t)
	a, b := feed("a", rss.Post{UUID: "shared"}), feed("b", rss.Post{UUID: "shared"})
	must(t, open(t, db, a, b).SetRead(rss.PostRef{FeedURL: "b", Key: "shared"}, true))
	got := open(t, db, a, b).Feeds()
	if got[0].Unread != 1 || got[1].Unread != 0 {
		t.Errorf("unread a %d b %d, want 1, 0", got[0].Unread, got[1].Unread)
	}
}

// rowCount is how many read rows db holds.
func rowCount(t *testing.T, db *storage.DB) int {
	t.Helper()
	saved, err := db.LoadPostReadStatuses()
	must(t, err)
	return len(saved)
}

func TestOnlyChangedRowsAreWritten(t *testing.T) {
	db := testDB(t)
	a := feed("a", rss.Post{UUID: "1"}, rss.Post{UUID: "2"}, rss.Post{UUID: "3"})
	b := feed("b", rss.Post{UUID: "1"}, rss.Post{UUID: "2"})
	l := open(t, db, a, b)
	must(t, l.SetRead(a.Posts[0].Ref("a"), true))
	// A repeated mark must not write: with the database closed, a write fails.
	must(t, db.Close())
	if err := l.SetRead(a.Posts[0].Ref("a"), true); err != nil {
		t.Errorf("a repeated mark wrote: %v", err)
	}

	db = testDB(t)
	l = open(t, db, a, b)
	must(t, l.MarkRead([]string{"a", "missing"}))
	if n := rowCount(t, db); n != 3 {
		t.Errorf("marking a feed read wrote %d rows, want its 3 posts only (nothing of feed b)", n)
	}
	if got := l.Feeds(); got[0].Unread != 0 || got[1].Unread != 2 {
		t.Errorf("unread a %d b %d, want 0, 2", got[0].Unread, got[1].Unread)
	}
	must(t, db.Close())
	if err := l.MarkRead([]string{"a"}); err != nil {
		t.Errorf("marking a read feed wrote: %v", err)
	}
}

// A refresh swaps a feed's posts for new ones; what is known about a post
// carries over by identity, and new posts are unread.
func TestMarksOutliveFeedReplacement(t *testing.T) {
	l := open(t, testDB(t), feed("a", rss.Post{UUID: "1"}))
	must(t, l.SetRead(rss.PostRef{FeedURL: "a", Key: "1"}, true))
	l.Replace(feed("a", rss.Post{UUID: "1"}, rss.Post{UUID: "2"}))
	if l.Unread() != 1 {
		t.Errorf("unread = %d, want 1 (post 2 is new)", l.Unread())
	}
}

// Posts are newest first; across feeds, posts published together keep the
// feeds' order, and each entry knows its feed and read state.
func TestPostsInOrder(t *testing.T) {
	t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	at := func(h int) time.Time { return t0.Add(-time.Duration(h) * time.Hour) }
	a := rss.Feed{Title: "A", URL: "a", Posts: []rss.Post{{UUID: "a0", Published: at(0)}, {UUID: "a2", Published: at(2)}}}
	b := rss.Feed{Title: "B", URL: "b", Posts: []rss.Post{{UUID: "b0", Published: at(0)}, {UUID: "b1", Published: at(1)}}}
	l := open(t, testDB(t), a, b)
	must(t, l.SetRead(rss.PostRef{FeedURL: "b", Key: "b1"}, true))

	keys := func(es []Entry) []string {
		var out []string
		for _, e := range es {
			out = append(out, e.Ref.Key)
		}
		return out
	}
	if got := keys(l.Posts("b")); !slices.Equal(got, []string{"b0", "b1"}) {
		t.Errorf("feed b: %q", got)
	}
	all := l.AllPosts()
	if got := keys(all); !slices.Equal(got, []string{"a0", "b0", "b1", "a2"}) {
		t.Errorf("all posts: %q", got)
	}
	if e := all[2]; e.Feed != "B" || !e.Read || all[0].Read {
		t.Errorf("entries should carry feed name and read state: %+v", all)
	}
	if l.Posts("missing") != nil {
		t.Error("an unknown feed has no posts")
	}
}
