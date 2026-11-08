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

// State is the read flag of every post that has ever been marked.
// A post never marked is unread.
type State struct {
	db   *storage.DB
	read map[string]map[string]bool // feed URL -> post key -> read
}

// New is an empty State saving to db.
func New(db *storage.DB) *State {
	return &State{db: db, read: map[string]map[string]bool{}}
}

// Restore adds saved marks to the state; marks made since they were read win.
func (s *State) Restore(saved []storage.PostReadStatus) {
	read := make(map[string]map[string]bool)
	for _, st := range saved {
		if read[st.FeedURL] == nil {
			read[st.FeedURL] = map[string]bool{}
		}
		read[st.FeedURL][st.Key] = st.Read
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

// Load restores everything saved.
func (s *State) Load() error {
	saved, err := s.db.LoadPostReadStatuses()
	if err != nil {
		return err
	}
	s.Restore(saved)
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
	if err := s.db.SavePostReadStatuses([]storage.PostReadStatus{{FeedURL: ref.FeedURL, Key: ref.Key, Read: read}}); err != nil {
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
	if err := s.db.SavePostReadStatuses(changed); err != nil {
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
