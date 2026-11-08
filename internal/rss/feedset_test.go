package rss

import "testing"

func twoFeeds() []Feed {
	return []Feed{
		{URL: "a", Posts: []Post{{UUID: "a1"}, {UUID: "shared"}}},
		{URL: "b", Posts: []Post{{UUID: "b1"}, {UUID: "shared"}}},
	}
}

func TestFeedSet_PostByRef(t *testing.T) {
	s := NewFeedSet(twoFeeds()...)
	if p, ok := s.Post(PostRef{"a", "a1"}); !ok || p.UUID != "a1" {
		t.Errorf("a/a1 = %+v, %v", p, ok)
	}
	if _, ok := s.Post(PostRef{"a", "b1"}); ok {
		t.Error("b1 is not in feed a")
	}
	if _, ok := s.Post(PostRef{"nope", "a1"}); ok {
		t.Error("unknown feed found")
	}
	if _, ok := s.Post(PostRef{"a", "missing"}); ok {
		t.Error("missing key found")
	}
}

// A replaced feed keeps its place, and refs into it find the new posts or
// nothing, never a neighbour.
func TestFeedSet_ReplaceKeepsOrderAndIdentity(t *testing.T) {
	s := NewFeedSet(twoFeeds()...)
	s.Replace(Feed{URL: "a", Title: "new", Posts: []Post{{UUID: "a2"}}})
	if got := s.URLs(); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("order = %v", got)
	}
	if _, ok := s.Post(PostRef{"a", "a1"}); ok {
		t.Error("a1 should be gone after the replacement")
	}
	if _, ok := s.Post(PostRef{"a", "a2"}); !ok {
		t.Error("a2 should be found")
	}
	s.Replace(Feed{URL: "c"})
	if got := s.URLs(); len(got) != 3 || got[2] != "c" {
		t.Errorf("a new feed goes last: %v", got)
	}
}

// An untitled feed is named by its URL's host, with the port kept and "www."
// dropped, whatever way it came to have no title.
func TestFeedName(t *testing.T) {
	tests := []struct {
		feed Feed
		want string
	}{
		{Feed{Title: "Blog", URL: "https://www.example.com/feed"}, "Blog"},
		{Feed{Title: "  ", URL: "https://www.example.com:8080/feed"}, "example.com:8080"},
		{Feed{URL: "https://www.danluu.com/atom.xml"}, "danluu.com"},
		{Feed{URL: "https://sub.example.com/x"}, "sub.example.com"},
		{Feed{URL: "not a url"}, "not a url"},
	}
	for _, tt := range tests {
		if got := tt.feed.Name(); got != tt.want {
			t.Errorf("Name(%+v) = %q, want %q", tt.feed, got, tt.want)
		}
	}
}

func TestFeedName_SameForFailedAndColdFeeds(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	url := "https://www.example.invalid:8080/feed.xml"
	cold := store.Load([]string{url})[0]
	if err := db.SaveFeedCache(url, []byte("not a feed")); err != nil {
		t.Fatal(err)
	}
	broken := store.Load([]string{url})[0]
	for name, feed := range map[string]Feed{"cold": cold, "broken cache": broken} {
		if feed.Title != "" || feed.Name() != "example.invalid:8080" {
			t.Errorf("%s: title %q name %q", name, feed.Title, feed.Name())
		}
	}
}
