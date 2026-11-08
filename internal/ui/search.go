package ui

import (
	"bytes"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/isabelroses/izrss/internal/rss"
)

// searchIndex holds every post's lowercased title and content so each
// keystroke is a plain substring scan rather than re-lowercasing every post.
// It is kept per feed, so a refreshed feed is re-indexed on its own.
type searchIndex struct {
	feeds []feedIndex    // in feed order
	pos   map[string]int // feed URL -> place in feeds; private to the index
	// Every post, newest first (ties in feed and post order); a post's
	// indexedPost.pos is its place here.
	order []postRef
	spare []postRef // the order before the last update, reused by the next

	// The last non-blank search: its needle and the posts it matched, in
	// feed order. A query that extends it can only match a subset.
	last     []byte
	lastHits []place

	marks []uint8 // by order position: a hit's rank+1, else 0; scratch
}

// How well a post matches, best first.
const (
	rankTitleWord = iota // the title has it at the start of a word
	rankTitle            // the title has it mid-word
	rankContent          // only the content has it
	rankCount
	noMatch = -1
)

// feedIndex is one feed's posts: post j's title is text[posts[j].start:
// posts[j].mid] and its content text[posts[j].mid:posts[j].end].
type feedIndex struct {
	url   string
	text  []byte
	posts []indexedPost
}

type indexedPost struct {
	key             string // the post's rss.Post.Key()
	start, mid, end int
	pos             int // where the post is in searchIndex.order
}

// place is a post's position in the feeds the index was built from. It never
// leaves the index: results are handed out as rss.PostRef.
type place struct{ feed, post int }

// newSearchIndex indexes every feed into one shared buffer and posts slice.
func newSearchIndex(feeds []rss.Feed) *searchIndex {
	n, size := 0, 0
	for _, feed := range feeds {
		n += len(feed.Posts)
		size += textSize(feed)
	}
	text, posts := make([]byte, 0, size), make([]indexedPost, n)
	idx := &searchIndex{feeds: make([]feedIndex, len(feeds)), pos: make(map[string]int, len(feeds))}
	for i, feed := range feeds {
		start := len(text)
		ps := posts[:len(feed.Posts):len(feed.Posts)]
		posts = posts[len(feed.Posts):]
		text = indexFeed(text, ps, feed)
		idx.feeds[i] = feedIndex{url: feed.URL, text: text[start:len(text):len(text)], posts: ps}
		idx.pos[feed.URL] = i
	}
	idx.order = make([]postRef, 0, n)
	for i := range feeds {
		idx.order = appendRefs(idx.order, feeds, i)
	}
	idx.order = sortNewestFirst(idx.order)
	idx.placeOrder()
	return idx
}

// placeOrder records each post's position in idx.order.
func (idx *searchIndex) placeOrder() {
	for n, r := range idx.order {
		idx.feeds[r.feed].posts[r.post].pos = n
	}
	idx.marks = slices.Grow(idx.marks[:0], len(idx.order))[:len(idx.order)]
}

// appendRefs appends a postRef for each of feeds[i]'s posts.
func appendRefs(refs []postRef, feeds []rss.Feed, i int) []postRef {
	for j, p := range feeds[i].Posts {
		refs = append(refs, postRef{sec: p.Published.Unix(), nsec: int32(p.Published.Nanosecond()), feed: int32(i), post: int32(j)})
	}
	return refs
}

func textSize(feed rss.Feed) int {
	size := 0
	for _, post := range feed.Posts {
		size += len(post.Title) + len(post.Content)
	}
	return size
}

// indexFeed appends feed's lowercased posts to text, recording in posts
// where each lies relative to where the feed's text starts.
func indexFeed(text []byte, posts []indexedPost, feed rss.Feed) []byte {
	base := len(text)
	for j, post := range feed.Posts {
		p := &posts[j]
		p.key = post.Key()
		p.start = len(text) - base
		text = appendLower(text, post.Title)
		p.mid = len(text) - base
		text = appendLower(text, post.Content)
		p.end = len(text) - base
	}
	return text
}

// update re-indexes the feed with url after it changed, as
// newSearchIndex(feeds) would index it. The feed may have been appended
// after the index was built.
func (idx *searchIndex) update(feeds []rss.Feed, url string) {
	i, known := idx.pos[url]
	switch {
	case !known:
		i = len(idx.feeds)
		if len(feeds) != i+1 || feeds[i].URL != url {
			*idx = *newSearchIndex(feeds)
			return
		}
		idx.feeds = append(idx.feeds, feedIndex{})
		idx.pos[url] = i
	case len(feeds) != len(idx.feeds) || feeds[i].URL != url:
		*idx = *newSearchIndex(feeds)
		return
	}
	posts := make([]indexedPost, len(feeds[i].Posts))
	idx.feeds[i] = feedIndex{url: url, text: indexFeed(make([]byte, 0, textSize(feeds[i])), posts, feeds[i]), posts: posts}
	idx.last, idx.lastHits = nil, nil // the last hits may have changed

	// Merge feed i's posts back into the order in one pass, as the sort in
	// newSearchIndex would place them: by time, ties in feed order.
	fresh := sortNewestFirst(appendRefs(nil, feeds, i))
	merged := slices.Grow(idx.spare[:0], len(idx.order)+len(fresh))
	for _, r := range idx.order {
		if int(r.feed) == i {
			continue
		}
		for len(fresh) > 0 && (fresh[0].newer(r) || !r.newer(fresh[0]) && i < int(r.feed)) {
			merged, fresh = append(merged, fresh[0]), fresh[1:]
		}
		merged = append(merged, r)
	}
	idx.order, idx.spare = append(merged, fresh...), idx.order
	idx.placeOrder()
}

// search returns every post whose title or content contains query,
// case-insensitively: title matches at a word start, then other title
// matches, then content-only matches, each newest first. An empty query
// matches nothing.
func (idx *searchIndex) search(query string) []rss.PostRef {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	needle := []byte(strings.ToLower(query))
	// Posts are checked in feed order, which reads the text sequentially;
	// each hit's rank is marked at its place in the recency order, and a
	// pass over that order lays out the results.
	var counts [rankCount]int
	found := idx.lastHits[:0] // filtered in place: found never overtakes the read
	check := func(loc place) {
		fi := &idx.feeds[loc.feed]
		if rank := fi.match(loc.post, needle); rank != noMatch {
			found = append(found, loc)
			idx.marks[fi.posts[loc.post].pos] = uint8(rank + 1)
			counts[rank]++
		}
	}
	// Text containing needle contains any part of it, so when the last
	// needle is part of this one only its hits need checking.
	if idx.last != nil && bytes.Contains(needle, idx.last) {
		for _, loc := range idx.lastHits {
			check(loc)
		}
	} else {
		for i := range idx.feeds {
			for j := range idx.feeds[i].posts {
				check(place{i, j})
			}
		}
	}
	idx.last, idx.lastHits = needle, found
	if len(found) == 0 {
		return nil
	}
	var next [rankCount]int // where each rank's next hit goes
	for r := 1; r < rankCount; r++ {
		next[r] = next[r-1] + counts[r-1]
	}
	hits := make([]rss.PostRef, len(found)) // the caller's copy; the rest is reused
	for n, mark := range idx.marks {
		if mark != 0 {
			r := &idx.order[n]
			fi := &idx.feeds[r.feed]
			hits[next[mark-1]] = rss.PostRef{FeedURL: fi.url, Key: fi.posts[r.post].key}
			next[mark-1]++
			idx.marks[n] = 0
		}
	}
	return hits
}

// match ranks how post j's title or content contains needle.
func (fi *feedIndex) match(j int, needle []byte) int {
	p := fi.posts[j]
	title := fi.text[p.start:p.mid]
	if at := bytes.Index(title, needle); at >= 0 {
		for {
			if wordStart(title, at) {
				return rankTitleWord
			}
			next := bytes.Index(title[at+1:], needle)
			if next < 0 {
				return rankTitle
			}
			at += 1 + next
		}
	}
	if bytes.Contains(fi.text[p.mid:p.end], needle) {
		return rankContent
	}
	return noMatch
}

// wordStart reports whether s[at:] starts a word: nothing before it, or no
// letter or digit.
func wordStart(s []byte, at int) bool {
	if at == 0 {
		return true
	}
	if c := s[at-1]; c < utf8.RuneSelf {
		return !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9')
	}
	r, _ := utf8.DecodeLastRune(s[:at])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

// appendLower appends the lowercase form of s to dst, producing exactly the
// bytes strings.ToLower(s) would, but into a reusable buffer. Runs of bytes
// that lowering leaves alone are copied in one go.
func appendLower(dst []byte, s string) []byte {
	for i := 0; i < len(s); {
		j := i
		for j < len(s) {
			c := s[j]
			if c < utf8.RuneSelf {
				if 'A' <= c && c <= 'Z' {
					break
				}
				j++
				continue
			}
			// General Punctuation (dashes, curly quotes, …) has no case.
			r, size := utf8.DecodeRuneInString(s[j:])
			if r < 0x2000 || r > 0x206f {
				break
			}
			j += size
		}
		dst = append(dst, s[i:j]...)
		if i = j; i >= len(s) {
			break
		}
		if c := s[i]; c < utf8.RuneSelf {
			dst = append(dst, c+('a'-'A'))
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		dst = utf8.AppendRune(dst, unicode.ToLower(r))
		i += size
	}
	return dst
}
