package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isabelroses/izrss/internal/rss"
)

func TestAppendLower_MatchesStringsToLower(t *testing.T) {
	inputs := []string{
		"",
		"plain ascii",
		"MiXeD CaSe 123",
		"Ünïcödé — “QUOTES”",
		"İSTANBUL",          // İ lowers to a 2-byte i̇ form
		"K Kelvin",          // Kelvin sign lowers to ASCII k
		"ΣΊΣΥΦΟΣ",           // Greek capitals
		"bad \xff\xfe utf8", // invalid bytes become U+FFFD
		"trunc\xe2\x80",     // a General Punctuation prefix, cut short
		"Ⅻ Ⓐ ǅ",             // letterlike, enclosed and titlecase forms
	}
	for _, s := range inputs {
		t.Run(s, func(t *testing.T) {
			if got, want := string(appendLower(nil, s)), strings.ToLower(s); got != want {
				t.Errorf("appendLower(%q) = %q, want %q", s, got, want)
			}
		})
	}
}

// Lowering leaves every General Punctuation rune alone, as appendLower's
// fast path assumes.
func TestAppendLower_GeneralPunctuationHasNoCase(t *testing.T) {
	for r := rune(0x2000); r <= 0x206f; r++ {
		if s := "A" + string(r) + "B"; string(appendLower(nil, s)) != strings.ToLower(s) {
			t.Errorf("U+%04X lowers differently", r)
		}
	}
}

// numbered gives each feed without one a distinct URL, as real feeds have.
func numbered(feeds rss.Feeds) rss.Feeds {
	for i := range feeds {
		if feeds[i].URL == "" {
			feeds[i].URL = fmt.Sprintf("https://feed-%d.example", i)
		}
	}
	return feeds
}

// titleOf is the title of the post ref addresses.
func titleOf(feeds rss.Feeds, ref rss.PostRef) string {
	p, _ := rss.NewFeedSet(feeds...).Post(ref)
	return p.Title
}

func TestSearchPosts(t *testing.T) {
	feeds := rss.Feeds{
		{Posts: []rss.Post{
			{Title: "a", Content: "Go is FUN"},
			{Title: "b", Content: "Rust"},
			{Title: "Weekly digest", Content: "news"},
		}},
		{Posts: []rss.Post{
			{Title: "c", Content: "Kelvin scale"},
		}},
	}
	tests := []struct {
		query string
		want  []string
	}{
		{"fun", []string{"a"}},
		{"FUN", []string{"a"}},
		{"kelvin", []string{"c"}},
		{"", nil},
		{"weekly", []string{"Weekly digest"}},
		{"python", nil},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			var got []string
			for _, h := range newSearchIndex(numbered(feeds)).search(tt.query) {
				got = append(got, titleOf(feeds, h))
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("search(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

// Searching one index query after query, as typing does, finds what a fresh
// index would, whether the query grows, shrinks or changes.
func TestSearchIndex_Incremental(t *testing.T) {
	feeds := benchFeeds()
	feeds[3].Posts[5].Title = "The Quick Fix"
	idx := newSearchIndex(numbered(feeds))
	for _, q := range []string{"q", "qu", "quick", "quick ", "quick f", "QUICK FOX", "quick", "", "k", "ick", "haystack", "needle in", "zzz", "zzzz"} {
		got, want := idx.search(q), newSearchIndex(numbered(feeds)).search(q)
		if !slices.Equal(got, want) {
			t.Errorf("search(%q): %d hits, want %d", q, len(got), len(want))
		}
	}
}

// Re-indexing one changed or appended feed searches as a fresh index does.
func TestSearchIndex_Update(t *testing.T) {
	feeds := benchFeeds()
	idx := newSearchIndex(numbered(feeds))
	_ = idx.search("needle") // leave narrowing state behind

	feeds[2].Posts = []rss.Post{{Title: "Fresh NEEDLE"}, {Content: "no match"}}
	idx.update(feeds, feeds[2].URL)
	feeds = append(feeds, rss.Feed{URL: "appended", Posts: []rss.Post{{Content: "needle again"}}})
	idx.update(feeds, "appended")

	for _, q := range []string{"needle", "needle ", "fresh", "haystack", "zzz"} {
		got, want := idx.search(q), newSearchIndex(numbered(feeds)).search(q)
		if !slices.Equal(got, want) {
			t.Errorf("search(%q) = %v, want %v", q, got, want)
		}
	}
}

// Title matches rank above content-only ones, and a title match at a word
// start above one mid-word; within a rank, newer posts come first.
func TestSearch_RanksTitleMatches(t *testing.T) {
	at := func(h int) time.Time { return t0.Add(-time.Duration(h) * time.Hour) }
	feeds := rss.Feeds{
		{Posts: []rss.Post{
			{Title: "Weekly links", Content: "<p>the release went out</p>", Published: at(0)},
			{Title: "Prerelease builds", Content: "x", Published: at(1)},
			{Title: "Release notes 1.2", Content: "x", Published: at(5)},
		}},
		{Posts: []rss.Post{
			{Title: "On shipping", Content: "<p>RELEASE early</p>", Published: at(2)},
			{Title: "Release notes 1.3", Content: "x", Published: at(3)},
			{Title: "Unreleased ideas", Content: "x", Published: at(4)},
		}},
	}
	want := []string{"Release notes 1.3", "Release notes 1.2", "Prerelease builds", "Unreleased ideas", "Weekly links", "On shipping"}
	idx := newSearchIndex(numbered(feeds))
	for _, q := range []string{"rel", "release"} { // fresh, then narrowed
		var got []string
		for _, h := range idx.search(q) {
			got = append(got, titleOf(feeds, h))
		}
		if !slices.Equal(got, want) {
			t.Errorf("search(%q) = %q, want %q", q, got, want)
		}
	}
}

// Re-indexing a feed keeps the recency order a fresh index would have,
// including posts tied on time.
func TestSearchIndex_UpdateKeepsRecencyOrder(t *testing.T) {
	feeds := make(rss.Feeds, 4)
	for i := range feeds {
		for j := range 5 {
			feeds[i].Posts = append(feeds[i].Posts, rss.Post{Title: fmt.Sprintf("post %d.%d", i, j), Published: t0.Add(-time.Duration(j*2+i%2) * time.Hour)})
		}
	}
	idx := newSearchIndex(numbered(feeds))
	feeds[2].Posts = []rss.Post{{Title: "post new", Published: t0.Add(time.Hour)}, {Title: "post tie", Published: t0.Add(-2 * time.Hour)}, {Title: "post old", Published: t0.Add(-time.Hour * 99)}}
	idx.update(feeds, feeds[2].URL)
	feeds = append(feeds, rss.Feed{URL: "appended", Posts: []rss.Post{{Title: "post appended", Published: t0}}})
	idx.update(feeds, "appended")
	if got, want := idx.search("post"), newSearchIndex(numbered(feeds)).search("post"); !slices.Equal(got, want) {
		t.Errorf("after update: %v, want %v", got, want)
	}
}
