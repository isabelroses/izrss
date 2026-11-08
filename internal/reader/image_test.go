package reader

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

const imgSrc = "https://example.com/a.png"

func TestImage(t *testing.T) {
	tests := []struct {
		name       string
		html       string
		show       bool
		ready      []string
		failed     bool
		wantLines  []string // stripped
		wantHeight int
	}{
		{name: "images off", html: `<img src="a.png" alt="cat">`, wantLines: []string{"cat"}, wantHeight: 1},
		{name: "pending", html: `<img src="a.png" alt="cat">`, show: true, wantLines: []string{"[image: cat]"}, wantHeight: 1},
		{name: "ready", html: `<img src="a.png" alt="cat">`, show: true, ready: []string{"AAA", "BBB"}, wantLines: []string{"AAA", "BBB"}, wantHeight: 2},
		{name: "failed", html: `<img src="a.png" alt="cat">`, show: true, failed: true, wantLines: []string{"cat"}, wantHeight: 1},
		{name: "empty alt", html: `<img src="a.png">`, wantLines: []string{"image"}, wantHeight: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := opts(80)
			o.BaseURL = "https://example.com/"
			o.ShowImages = tt.show
			if tt.ready != nil {
				o.Ready = map[string][]string{imgSrc: tt.ready}
			}
			if tt.failed {
				o.Failed = map[string]bool{imgSrc: true}
			}
			doc := Render(tt.html, o)

			if got := plain(doc); strings.Join(got, "\n") != strings.Join(tt.wantLines, "\n") {
				t.Errorf("lines = %q, want %q", got, tt.wantLines)
			}
			if len(doc.Images) != 1 {
				t.Fatalf("expected 1 image, got %+v", doc.Images)
			}
			img := doc.Images[0]
			if img.Src != imgSrc || img.Height != tt.wantHeight || img.Line != 0 {
				t.Errorf("image = %+v", img)
			}
		})
	}
}

func TestImage_AltIsHyperlinkedWhenNotDrawn(t *testing.T) {
	o := opts(80)
	o.BaseURL = "https://example.com/"
	out := Render(`<img src="a.png" alt="cat">`, o).String()
	if !strings.Contains(out, ansi.SetHyperlink(imgSrc)) {
		t.Errorf("alt text should link to the image: %q", out)
	}
}

func TestImage_ReadyRowsVerbatim(t *testing.T) {
	o := opts(80)
	o.BaseURL = "https://example.com/"
	o.ShowImages = true
	row := "\x1b[38;5;7m\U0010EEEE\x1b[m"
	o.Ready = map[string][]string{imgSrc: {row}}
	if doc := Render(`<p>x</p><img src="a.png">`, o); doc.Lines[2] != row {
		t.Errorf("placeholder rows must not be restyled: %q", doc.Lines[2])
	}
}

func TestImage_SplitsParagraph(t *testing.T) {
	lines := plain(Render(`<p>before <img src="x.png" alt="pic"> after</p>`, opts(80)))
	want := []string{"before", "", "pic", "", "after"}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q, want %q", lines, want)
	}
}

func TestImage_RelativeSrcResolvedAgainstBase(t *testing.T) {
	o := opts(80)
	o.BaseURL = "https://x.dev/posts/1/"
	doc := Render(`<img src="../img/a.png">`, o)
	if len(doc.Images) != 1 || doc.Images[0].Src != "https://x.dev/posts/img/a.png" {
		t.Errorf("got %+v", doc.Images)
	}
}

func TestImage_TrackingPixelSkipped(t *testing.T) {
	doc := Render(`<p>text</p><img src="https://t.co/p.gif" width="1" height="1">`, opts(80))
	if len(doc.Images) != 0 || len(doc.Lines) != 1 {
		t.Errorf("tracking pixel should vanish: lines %q images %+v", plain(doc), doc.Images)
	}
}

func TestImage_DataAndNonHTTPSkipped(t *testing.T) {
	for _, src := range []string{"data:image/png;base64,AAAA", "javascript:alert(1)"} {
		o := opts(80)
		o.ShowImages = true
		doc := Render(`<img src="`+src+`" alt="inline">`, o)
		if len(doc.Images) != 0 {
			t.Errorf("%s should not be listed: %+v", src, doc.Images)
		}
		if got := plain(doc); len(got) != 1 || got[0] != "inline" {
			t.Errorf("%s should show alt text: %q", src, got)
		}
	}
}

func TestImage_LineIndexes(t *testing.T) {
	o := opts(80)
	o.BaseURL = "https://example.com/"
	o.ShowImages = true
	o.Ready = map[string][]string{"https://example.com/b.png": {"B1", "B2", "B3"}}
	doc := Render(`<p>one</p><img src="a.png"><p>two</p><ul><li><img src="b.png"></li></ul><blockquote><img src="c.png"></blockquote>`, o)
	if len(doc.Images) != 3 {
		t.Fatalf("got %+v", doc.Images)
	}
	for _, img := range doc.Images {
		first := ansi.Strip(doc.Lines[img.Line])
		switch {
		case strings.HasSuffix(img.Src, "a.png") && first != "[image: image]",
			strings.HasSuffix(img.Src, "b.png") && !strings.HasSuffix(first, "B1"),
			strings.HasSuffix(img.Src, "c.png") && !strings.HasSuffix(first, "[image: image]"):
			t.Errorf("%s Line=%d points at %q", img.Src, img.Line, first)
		}
	}
}

func TestImage_WidthAndSrcset(t *testing.T) {
	o := opts(80)
	o.BaseURL = "https://x.dev/posts/1/"
	doc := Render(`<picture><source type="image/avif" srcset="a-640.avif 640w"><img src="a-640.png" width="914" height="898" srcset="a-640.png 640w, /img/a-914.png 914w, a-2x.png 2x"></picture>`, o)
	if len(doc.Images) != 1 {
		t.Fatalf("got %+v", doc.Images)
	}
	img := doc.Images[0]
	if img.Src != "https://x.dev/posts/1/a-640.png" || img.Width != 914 {
		t.Errorf("src/width = %q/%d", img.Src, img.Width)
	}
	want := []Candidate{{URL: "https://x.dev/posts/1/a-640.png", W: 640}, {URL: "https://x.dev/img/a-914.png", W: 914}}
	if len(img.Srcset) != len(want) {
		t.Fatalf("srcset = %+v, want %+v (x descriptors skipped)", img.Srcset, want)
	}
	for i := range want {
		if img.Srcset[i] != want[i] {
			t.Errorf("srcset[%d] = %+v, want %+v", i, img.Srcset[i], want[i])
		}
	}
}

func TestImage_PercentWidthIgnored(t *testing.T) {
	if doc := Render(`<img src="https://x.dev/a.png" width="100%">`, opts(80)); doc.Images[0].Width != 0 {
		t.Errorf("non-pixel width should be 0, got %d", doc.Images[0].Width)
	}
}

// Alt text is attribute text and may hold newlines; each must still render
// as exactly one line, or Image.Line and per-line widths drift.
func TestImage_MultilineAltStaysOneLine(t *testing.T) {
	const html = "<p>before</p><img src=\"a.png\" alt=\"  a\ncat\r\n\ton\fa  mat \"><p>after</p>"
	for _, mode := range []struct {
		name         string
		show, failed bool
		base         string
	}{
		{name: "images off", base: "https://example.com/"},
		{name: "pending", show: true, base: "https://example.com/"},
		{name: "failed", show: true, failed: true, base: "https://example.com/"},
		{name: "not http"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			o := opts(80)
			o.BaseURL = mode.base
			o.ShowImages = mode.show
			if mode.failed {
				o.Failed = map[string]bool{imgSrc: true}
			}
			doc := Render(html, o)
			for i, l := range doc.Lines {
				if strings.ContainsAny(l, "\n\r\t\f") {
					t.Errorf("line %d holds a line break or tab: %q", i, l)
				}
			}
			if !strings.Contains(strings.Join(plain(doc), "\n"), "a cat on a mat") {
				t.Errorf("alt whitespace should collapse to single spaces: %q", plain(doc))
			}
			if len(doc.Images) == 1 && plain(doc)[doc.Images[0].Line] == "" {
				t.Errorf("Image.Line %d points at a blank line: %q", doc.Images[0].Line, plain(doc))
			}
		})
	}
}
