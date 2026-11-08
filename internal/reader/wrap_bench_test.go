package reader

import (
	"os"
	"testing"
)

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
