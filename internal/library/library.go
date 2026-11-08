// Package library is the feeds izrss shows and what has been read of them:
// loading both at startup, keeping read marks saved, and answering what the
// screens ask (feeds with unread counts, posts in order, search). Every
// change bumps Version, so a view built from the library knows to rebuild
// without each change site having to say so.
package library

import (
	"slices"
	"time"

	"github.com/isabelroses/izrss/internal/rss"
	"github.com/isabelroses/izrss/internal/storage"
)

// Library is the feeds in order and the read state of their posts.
type Library struct {
	feeds   *rss.FeedSet
	store   *rss.Store
	marks   marks
	version int
}

// New is an empty library loading feeds from store and keeping read marks
// in db.
func New(db *storage.DB, store *rss.Store) *Library {
	return &Library{feeds: rss.NewFeedSet(), store: store, marks: marks{db: db, read: map[string]map[string]bool{}}}
}

// Snapshot is what Load read: the cached feeds and the saved read marks.
type Snapshot struct {
	Feeds    []rss.Feed
	marks    []storage.PostReadStatus
	MarksErr error // the read marks couldn't be loaded; the feeds still are
}

// Fetched is when the latest of the snapshot's feeds was fetched, the zero
// time if none was.
func (s Snapshot) Fetched() time.Time {
	var at time.Time
	for _, f := range s.Feeds {
		if f.Fetched.After(at) {
			at = f.Fetched
		}
	}
	return at
}

// Load reads the cached feeds with urls and the saved read marks without
// changing the library, so it may run off the main loop; hand the result to
// Restore.
func (l *Library) Load(urls []string) Snapshot {
	rows, err := l.marks.db.LoadPostReadStatuses()
	return Snapshot{Feeds: l.store.Load(urls), marks: rows, MarksErr: err}
}

// Restore replaces every feed with the snapshot's and adds its read marks;
// marks made since it was loaded win.
func (l *Library) Restore(s Snapshot) {
	l.feeds.Reset(s.Feeds)
	l.marks.restore(s.marks)
	l.version++
}

// Replace swaps in f for the feed with its URL; a new feed goes last.
func (l *Library) Replace(f rss.Feed) {
	l.feeds.Replace(f)
	l.version++
}

// Version changes whenever the feeds or their read state do.
func (l *Library) Version() int { return l.version }

// Len is the number of feeds.
func (l *Library) Len() int { return l.feeds.Len() }

// URLs lists the feed URLs in order.
func (l *Library) URLs() []string { return l.feeds.URLs() }

// Feed finds the feed with url.
func (l *Library) Feed(url string) (rss.Feed, bool) { return l.feeds.Feed(url) }

// FeedInfo is a feed as a list shows it.
type FeedInfo struct {
	URL, Name string
	Unread    int
}

// Feeds lists the feeds in order, with their unread counts.
func (l *Library) Feeds() []FeedInfo {
	all := l.feeds.All()
	out := make([]FeedInfo, len(all))
	for i, f := range all {
		out[i] = FeedInfo{URL: f.URL, Name: f.Name(), Unread: l.marks.unread(f)}
	}
	return out
}

// Unread counts the unread posts of every feed.
func (l *Library) Unread() int {
	n := 0
	for _, f := range l.feeds.All() {
		n += l.marks.unread(f)
	}
	return n
}

// Entry is a post with where it is and whether it has been read.
type Entry struct {
	Ref  rss.PostRef
	Post rss.Post
	Feed string // the name of the post's feed
	Read bool
}

func (l *Library) entry(f rss.Feed, p rss.Post) Entry {
	ref := p.Ref(f.URL)
	return Entry{Ref: ref, Post: p, Feed: f.Name(), Read: l.marks.isRead(ref)}
}

// Entry finds the post ref addresses.
func (l *Library) Entry(ref rss.PostRef) (Entry, bool) {
	f, ok := l.feeds.Feed(ref.FeedURL)
	if !ok {
		return Entry{}, false
	}
	p, ok := l.feeds.Post(ref)
	if !ok {
		return Entry{}, false
	}
	return l.entry(f, p), true
}

// Posts lists the posts of the feed with url, newest first.
func (l *Library) Posts(url string) []Entry {
	f, ok := l.feeds.Feed(url)
	if !ok {
		return nil
	}
	out := make([]Entry, len(f.Posts))
	for i, p := range f.Posts {
		out[i] = l.entry(f, p)
	}
	return out
}

// AllPosts lists every feed's posts, newest first; posts published at the
// same time keep the feeds' order.
func (l *Library) AllPosts() []Entry {
	var out []Entry
	for _, f := range l.feeds.All() {
		for _, p := range f.Posts {
			out = append(out, l.entry(f, p))
		}
	}
	slices.SortStableFunc(out, func(a, b Entry) int { return b.Post.Published.Compare(a.Post.Published) })
	return out
}

// Search lists the posts whose title or content contains query, best match
// first; see searchPosts.
func (l *Library) Search(query string) []rss.PostRef { return searchPosts(l.feeds.All(), query) }

// IsRead reports whether the post ref addresses is read.
func (l *Library) IsRead(ref rss.PostRef) bool { return l.marks.isRead(ref) }

// SetRead marks the post ref addresses read or unread and saves it. A post
// not in the library, or already so, is left alone. The mark holds even when
// saving fails.
func (l *Library) SetRead(ref rss.PostRef, read bool) error {
	if _, ok := l.feeds.Post(ref); !ok || l.marks.isRead(ref) == read {
		return nil
	}
	l.version++
	return l.marks.set(ref, read)
}

// MarkRead marks every post of the feeds with urls read, saving only those
// that were not.
func (l *Library) MarkRead(urls []string) error {
	var feeds []rss.Feed
	for _, url := range urls {
		if f, ok := l.feeds.Feed(url); ok {
			feeds = append(feeds, f)
		}
	}
	l.version++
	return l.marks.markFeedsRead(feeds)
}
