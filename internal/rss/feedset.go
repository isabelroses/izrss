package rss

// PostRef addresses a post by identity: the feed it is in and its Key. Unlike
// a position it stays valid across refreshes, and finds nothing, rather than
// the wrong post, once the post is gone.
type PostRef struct{ FeedURL, Key string }

// Ref is the address of p in the feed with feedURL.
func (p Post) Ref(feedURL string) PostRef { return PostRef{FeedURL: feedURL, Key: p.Key()} }

// FeedSet is the ordered feeds, addressed by feed URL and post key. Feeds
// keep the place they were first given, however often they are replaced, so
// callers never hold a position.
type FeedSet struct {
	feeds []Feed
	pos   map[string]int // feed URL -> place in feeds; internal
}

// NewFeedSet holds feeds in the given order. A repeated URL keeps its first.
func NewFeedSet(feeds ...Feed) *FeedSet {
	s := &FeedSet{}
	s.Reset(feeds)
	return s
}

// Reset replaces every feed.
func (s *FeedSet) Reset(feeds []Feed) {
	s.feeds = make([]Feed, 0, len(feeds))
	s.pos = make(map[string]int, len(feeds))
	for _, f := range feeds {
		if _, dup := s.pos[f.URL]; !dup {
			s.pos[f.URL] = len(s.feeds)
			s.feeds = append(s.feeds, f)
		}
	}
}

// Len is the number of feeds.
func (s *FeedSet) Len() int { return len(s.feeds) }

// All lists the feeds in order. The slice is a view for iterating within one
// call: do not keep it or index into it across a refresh.
func (s *FeedSet) All() []Feed { return s.feeds }

// URLs lists the feed URLs in order.
func (s *FeedSet) URLs() []string {
	urls := make([]string, len(s.feeds))
	for i, f := range s.feeds {
		urls[i] = f.URL
	}
	return urls
}

// Feed finds the feed with url.
func (s *FeedSet) Feed(url string) (Feed, bool) {
	i, ok := s.pos[url]
	if !ok {
		return Feed{}, false
	}
	return s.feeds[i], true
}

// Post finds the post ref addresses.
func (s *FeedSet) Post(ref PostRef) (Post, bool) {
	i, ok := s.pos[ref.FeedURL]
	if !ok {
		return Post{}, false
	}
	for _, p := range s.feeds[i].Posts {
		if p.Key() == ref.Key {
			return p, true
		}
	}
	return Post{}, false
}

// Replace swaps in f for the feed with its URL, in place; a feed not yet in
// the set goes last.
func (s *FeedSet) Replace(f Feed) {
	if i, ok := s.pos[f.URL]; ok {
		s.feeds[i] = f
		return
	}
	if s.pos == nil {
		s.pos = map[string]int{}
	}
	s.pos[f.URL] = len(s.feeds)
	s.feeds = append(s.feeds, f)
}
