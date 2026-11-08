package rss

import (
	"encoding/json"
	"fmt"

	"github.com/isabelroses/izrss/internal/storage"
)

// Store caches the last fetch of every feed, parsed, so a start shows the
// feeds without the network or a parse.
type Store struct{ db *storage.DB }

func NewStore(db *storage.DB) *Store { return &Store{db: db} }

// stored is a feed as the store saves it.
type stored struct {
	Title string `json:"title"`
	Posts []Post `json:"posts"`
}

// Save caches f as fetched now.
func (s *Store) Save(f Feed) error {
	data, err := json.Marshal(stored{Title: f.Title, Posts: f.Posts})
	if err != nil {
		return err
	}
	return s.db.SaveFeedCache(f.URL, data)
}

// Load is the cached feeds with urls, in order. A feed never cached comes
// back empty, named by its host, with a zero Fetched; one that can't be read
// comes back empty with Err set.
func (s *Store) Load(urls []string) []Feed { return fetchAll(urls, s.load) }

func (s *Store) load(url string) Feed {
	c, ok, err := s.db.LoadFeedCache(url)
	switch {
	case err != nil:
		return failed(url, err)
	case !ok:
		return Feed{URL: url, Posts: []Post{}}
	}
	var f Feed
	if c.Parsed {
		f, err = decode(url, c.Content)
	} else { // cached before the store kept feeds parsed
		f, err = parse(url, c.Content)
	}
	if err != nil {
		return failed(url, err)
	}
	f.Fetched = c.FetchedAt
	return f
}

// decode is the feed at url as Save stored it in data.
func decode(url string, data []byte) (Feed, error) {
	var st stored
	if err := json.Unmarshal(data, &st); err != nil {
		return Feed{}, fmt.Errorf("reading cached %s: %w", url, err)
	}
	if st.Posts == nil {
		st.Posts = []Post{}
	}
	return Feed{Title: st.Title, URL: url, Posts: st.Posts}, nil
}
