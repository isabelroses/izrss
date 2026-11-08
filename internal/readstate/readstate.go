// Package readstate is the one place that knows whether a post is read: it
// owns the identity rule (feed URL and post key), the in-memory truth, and
// saving it. A feed refresh replaces a feed's posts without touching what is
// known about them, and only changed rows are ever written.
package readstate

import (
	"fmt"

	"github.com/isabelroses/izrss/internal/rss"
	"github.com/isabelroses/izrss/internal/storage"
)

// Store is where read state is kept; *storage.DB is the SQLite adapter.
type Store interface {
	LoadPostReadStatuses() (map[storage.ReadKey]bool, error)
	SavePostReadStatus(feedURL, key string, read bool) error
	SavePostReadStatuses([]storage.PostReadStatus) error
}

// State is the read flag of every post that has ever been marked.
// A post never marked is unread.
type State struct {
	store Store
	read  map[string]map[string]bool // feed URL -> post key -> read
}

// New is an empty State saving to store.
func New(store Store) *State {
	return &State{store: store, read: map[string]map[string]bool{}}
}

// Snapshot is the saved state, as read off the main loop by Read.
type Snapshot map[storage.ReadKey]bool

// Read loads everything saved. It touches no State, so it can run in a
// command; hand the result to Restore on the main loop.
func Read(store Store) (Snapshot, error) {
	saved, err := store.LoadPostReadStatuses()
	return Snapshot(saved), err
}

// Restore makes snap the state. Marks made since it was read are kept.
func (s *State) Restore(snap Snapshot) {
	read := make(map[string]map[string]bool)
	for k, v := range snap {
		m := read[k.FeedURL]
		if m == nil {
			m = map[string]bool{}
			read[k.FeedURL] = m
		}
		m[k.Key] = v
	}
	for url, m := range s.read {
		for key, v := range m {
			if read[url] == nil {
				read[url] = map[string]bool{}
			}
			read[url][key] = v
		}
	}
	s.read = read
}

// Load is Read then Restore, for callers off the main loop.
func (s *State) Load() error {
	snap, err := Read(s.store)
	if err != nil {
		return err
	}
	s.Restore(snap)
	return nil
}

// IsRead reports whether the post with key in the feed with feedURL is read.
func (s *State) IsRead(feedURL, key string) bool { return s.read[feedURL][key] }

// Unread counts the unread posts of f.
func (s *State) Unread(f rss.Feed) int {
	unread, read := 0, s.read[f.URL]
	for i := range f.Posts {
		if !read[f.Posts[i].Key()] {
			unread++
		}
	}
	return unread
}

// Set marks the post ref addresses read or unread and saves it. A post
// already in that state is not written. The mark holds in memory even when
// saving fails.
func (s *State) Set(ref rss.PostRef, read bool) error {
	if s.IsRead(ref.FeedURL, ref.Key) == read {
		return nil
	}
	s.mark(ref.FeedURL, ref.Key, read)
	if err := s.store.SavePostReadStatus(ref.FeedURL, ref.Key, read); err != nil {
		return fmt.Errorf("saving read status: %w", err)
	}
	return nil
}

// MarkFeedRead marks every post of f read, saving only those that were not.
func (s *State) MarkFeedRead(f rss.Feed) error {
	var changed []storage.PostReadStatus
	for i := range f.Posts {
		key := f.Posts[i].Key()
		if s.IsRead(f.URL, key) {
			continue
		}
		s.mark(f.URL, key, true)
		changed = append(changed, storage.PostReadStatus{FeedURL: f.URL, Key: key, Read: true})
	}
	if len(changed) == 0 {
		return nil
	}
	if err := s.store.SavePostReadStatuses(changed); err != nil {
		return fmt.Errorf("saving read status: %w", err)
	}
	return nil
}

func (s *State) mark(feedURL, key string, read bool) {
	m := s.read[feedURL]
	if m == nil {
		m = map[string]bool{}
		s.read[feedURL] = m
	}
	m[key] = read
}
