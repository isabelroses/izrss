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
	find, lower := indexFold, func(s string) string { return s }
	if !isASCII(needle) { // folding bytes can't match it
		find, lower = strings.Index, strings.ToLower
	}
	title := lower(p.Title)
	rank := noMatch
	for from := 0; from <= len(title); {
		i := find(title[from:], needle)
		if i < 0 {
			break
		}
		if wordStart(title, from+i) {
			return rankTitleWord
		}
		rank = rankTitle
		from += i + 1
	}
	if rank == noMatch && find(lower(p.Content), needle) >= 0 {
		return rankContent
	}
	return rank
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// indexFold is strings.Index(strings.ToLower(s), needle) for a lowercase
// ASCII needle, without lowercasing a copy of s.
func indexFold(s, needle string) int {
	first := needle[0]
	for i := 0; i+len(needle) <= len(s); i++ {
		if c := s[i]; c == first || c-'A' < 26 && c+32 == first {
			j := 1
			for j < len(needle) && (s[i+j] == needle[j] || s[i+j]-'A' < 26 && s[i+j]+32 == needle[j]) {
				j++
			}
			if j == len(needle) {
				return i
			}
		}
	}
	return -1
}

// wordStart reports whether s[at:] starts a word: nothing before it, or no
// letter or digit.
func wordStart(s string, at int) bool {
	r, _ := utf8.DecodeLastRuneInString(s[:at])
	return at == 0 || !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
