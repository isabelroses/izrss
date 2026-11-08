package reader

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func BenchmarkInlineWrap(b *testing.B) {
	in := benchParagraph()
	b.ReportAllocs()
	for b.Loop() {
		_ = in.wrap(80)
	}
}

func benchParagraph() *inline {
	src := `<p>Terminal user interfaces are having a moment. With libraries like <a href="https://example.com/bubbletea">Bubble Tea</a> and <a href="/lipgloss">Lip Gloss</a>, building <strong>delightful</strong> tools is easier than ever, and <em>people notice</em>. This paragraph talks about <code>tea.Model</code> and why the Elm architecture fits.</p>`
	doc, _ := html.Parse(strings.NewReader(src))
	r := newRenderer(Options{Width: 80, Theme: Theme{}}, 80)
	r.walkChildren(body(doc).FirstChild) // the <p>'s children, so nothing flushes
	if len(r.inline.pieces) == 0 {
		panic("empty bench paragraph")
	}
	return &r.inline
}
