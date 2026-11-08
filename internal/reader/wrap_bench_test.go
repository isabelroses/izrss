package reader

import (
	"os"
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

// BenchmarkRender renders a typical post (code blocks tagged with their
// language) and one whose code blocks name no language.
func BenchmarkRender(b *testing.B) {
	for _, tc := range []struct{ name, path string }{
		{"post", "../ui/testdata/post.html"},
		{"untagged", "testdata/untagged-code.html"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			src, err := os.ReadFile(tc.path)
			if err != nil {
				b.Fatal(err)
			}
			o := Options{Width: 80, Theme: testTheme(), BaseURL: "https://example.com/posts/1"}
			b.ReportAllocs()
			for b.Loop() {
				_ = Render(string(src), o)
			}
		})
	}
}
