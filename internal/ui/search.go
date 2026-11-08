package ui

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/isabelroses/izrss/internal/rss"
)

// searchIndex holds every post's lowercased title and content so each
// keystroke is a plain substring scan rather than re-lowercasing every post.
type searchIndex struct {
	text  []byte // all titles and contents, lowercased, back to back
	posts []indexedPost
}

// indexedPost locates a post's title in text[start:mid] and its content in
// text[mid:end].
type indexedPost struct {
	loc             rss.Loc
	start, mid, end int
}

func newSearchIndex(feeds rss.Feeds) *searchIndex {
	n, size := 0, 0
	for _, feed := range feeds {
		n += len(feed.Posts)
		for _, post := range feed.Posts {
			size += len(post.Title) + len(post.Content)
		}
	}
	idx := &searchIndex{posts: make([]indexedPost, 0, n), text: make([]byte, 0, size)}
	for i, feed := range feeds {
		for j, post := range feed.Posts {
			p := indexedPost{loc: rss.Loc{Feed: i, Post: j}, start: len(idx.text)}
			idx.text = appendLower(idx.text, post.Title)
			p.mid = len(idx.text)
			idx.text = appendLower(idx.text, post.Content)
			p.end = len(idx.text)
			idx.posts = append(idx.posts, p)
		}
	}
	return idx
}

// search returns every post whose title or content contains query,
// case-insensitively. An empty query matches nothing.
func (idx *searchIndex) search(query string) []rss.Loc {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	needle := []byte(strings.ToLower(query))
	var hits []rss.Loc
	for _, p := range idx.posts {
		if bytes.Contains(idx.text[p.start:p.mid], needle) || bytes.Contains(idx.text[p.mid:p.end], needle) {
			hits = append(hits, p.loc)
		}
	}
	return hits
}

// appendLower appends the lowercase form of s to dst, producing exactly the
// bytes strings.ToLower(s) would, but into a reusable buffer.
func appendLower(dst []byte, s string) []byte {
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			dst = append(dst, c)
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		dst = utf8.AppendRune(dst, unicode.ToLower(r))
		i += size
	}
	return dst
}
