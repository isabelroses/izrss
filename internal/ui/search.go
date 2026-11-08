package ui

import (
	"cmp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/isabelroses/izrss/internal/rss"
)

// How well a post matches, best first.
const (
	rankTitleWord = iota // the title has it at the start of a word
	rankTitle            // the title has it mid-word
	rankContent          // only the content has it
	noMatch
)

// searchPosts returns every post whose title or content contains query,
// case-insensitively: title matches at a word start, then other title
// matches, then content-only matches, each newest first. An empty query
// matches nothing.
func searchPosts(feeds []rss.Feed, query string) []rss.PostRef {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	needle := strings.ToLower(query)
	type hit struct {
		rank      int
		published time.Time
		ref       rss.PostRef
	}
	var hits []hit
	for _, f := range feeds {
		for _, p := range f.Posts {
			if rank := matchRank(p, needle); rank != noMatch {
				hits = append(hits, hit{rank, p.Published, p.Ref(f.URL)})
			}
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int {
		return cmp.Or(cmp.Compare(a.rank, b.rank), b.published.Compare(a.published))
	})
	refs := make([]rss.PostRef, len(hits))
	for i, h := range hits {
		refs[i] = h.ref
	}
	return refs
}

// matchRank ranks how p's title or content contains the lowercase needle.
func matchRank(p rss.Post, needle string) int {
	title := strings.ToLower(p.Title)
	rank := noMatch
	for from := 0; from <= len(title); {
		i := strings.Index(title[from:], needle)
		if i < 0 {
			break
		}
		if wordStart(title, from+i) {
			return rankTitleWord
		}
		rank = rankTitle
		from += i + 1
	}
	if rank == noMatch && strings.Contains(strings.ToLower(p.Content), needle) {
		return rankContent
	}
	return rank
}

// wordStart reports whether s[at:] starts a word: nothing before it, or no
// letter or digit.
func wordStart(s string, at int) bool {
	r, _ := utf8.DecodeLastRuneInString(s[:at])
	return at == 0 || !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
