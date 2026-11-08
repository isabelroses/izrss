package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/isabelroses/izrss/internal/rss"
)

var searchSink []rss.Loc

// benchFeeds builds 20 feeds of 40 posts with ~5KB of HTML each, including
// non-ASCII punctuation as real feed content usually has.
func benchFeeds() rss.Feeds {
	body := strings.Repeat("<p>The quick brown fox — “jumps” over the lazy dog’s back.</p>\n", 80)
	feeds := make(rss.Feeds, 20)
	for i := range feeds {
		posts := make([]rss.Post, 40)
		for j := range posts {
			posts[j] = rss.Post{Title: fmt.Sprintf("post %d", j), Content: body}
		}
		posts[0].Content += "<p>Needle in the HAYSTACK</p>"
		feeds[i] = rss.Feed{Posts: posts}
	}
	return feeds
}

func BenchmarkSearchPosts(b *testing.B) {
	feeds := benchFeeds()
	for _, q := range []string{"haystack", "zzz-no-match"} {
		b.Run(q, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				searchSink = newSearchIndex(feeds).search(q)
			}
		})
	}
}

// BenchmarkPaletteTyping opens the palette and types a query one key at a
// time, searching after each keystroke as the UI does.
func BenchmarkPaletteTyping(b *testing.B) {
	m := Model{feeds: benchFeeds(), palette: newPalette()}
	b.ReportAllocs()
	for b.Loop() {
		m.openPalette()
		for _, r := range "haystack" {
			m.paletteKey(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
		searchSink = m.palette.hits
	}
}
