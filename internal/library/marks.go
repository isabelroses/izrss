package library

import (
	"fmt"

	"github.com/isabelroses/izrss/internal/rss"
	"github.com/isabelroses/izrss/internal/storage"
)

// marks is the read flag of every post that has ever been marked, and
// saving it. A post is identified by its feed URL and key, so a refresh
// that replaces a feed's posts keeps what is known about them; a post never
// marked is unread. Only changed rows are ever written.
type marks struct {
	db   *storage.DB
	read map[string]map[string]bool // feed URL -> post key -> read
}

// restore adds saved marks; marks made since they were read win.
func (m *marks) restore(saved []storage.PostReadStatus) {
	since := m.read
	m.read = map[string]map[string]bool{}
	for _, st := range saved {
		m.mark(st.FeedURL, st.Key, st.Read)
	}
	for url, keys := range since {
		for key, v := range keys {
			m.mark(url, key, v)
		}
	}
}

func (m *marks) isRead(ref rss.PostRef) bool { return m.read[ref.FeedURL][ref.Key] }

// unread counts the unread posts of f.
func (m *marks) unread(f rss.Feed) int {
	unread, read := 0, m.read[f.URL]
	for i := range f.Posts {
		if !read[f.Posts[i].Key()] {
			unread++
		}
	}
	return unread
}

// set marks the post ref addresses and saves it, unless it is already so.
// The mark holds in memory even when saving fails.
func (m *marks) set(ref rss.PostRef, read bool) error {
	if m.isRead(ref) == read {
		return nil
	}
	m.mark(ref.FeedURL, ref.Key, read)
	return m.save([]storage.PostReadStatus{{FeedURL: ref.FeedURL, Key: ref.Key, Read: read}})
}

// markFeedsRead marks every post of feeds read, saving only those that were
// not, in one write.
func (m *marks) markFeedsRead(feeds []rss.Feed) error {
	var changed []storage.PostReadStatus
	for _, f := range feeds {
		for i := range f.Posts {
			ref := f.Posts[i].Ref(f.URL)
			if m.isRead(ref) {
				continue
			}
			m.mark(ref.FeedURL, ref.Key, true)
			changed = append(changed, storage.PostReadStatus{FeedURL: ref.FeedURL, Key: ref.Key, Read: true})
		}
	}
	if len(changed) == 0 {
		return nil
	}
	return m.save(changed)
}

func (m *marks) save(changed []storage.PostReadStatus) error {
	if err := m.db.SavePostReadStatuses(changed); err != nil {
		return fmt.Errorf("saving read status: %w", err)
	}
	return nil
}

func (m *marks) mark(feedURL, key string, read bool) {
	keys := m.read[feedURL]
	if keys == nil {
		keys = map[string]bool{}
		m.read[feedURL] = keys
	}
	keys[key] = read
}
