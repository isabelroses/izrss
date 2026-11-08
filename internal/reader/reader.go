// Package reader renders post HTML as width-wrapped, styled terminal lines.
// Every line is self-contained (styles close on the line) and at most Options.Width wide.
package reader

import (
	"image/color"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// Theme holds the colours the reader draws with.
type Theme struct {
	Text, Subtle, Heading, Link, Code, Quote color.Color
	Chip                                     color.Color // text on the heading colour (h1)
	CodeTheme                                string      // chroma style name
}

// Options controls a single render.
type Options struct {
	Width      int // the whole line, Margin included
	Margin     int // blank columns each side of the text, as glamour's document margin
	Theme      Theme
	BaseURL    string              // the post's link; resolves relative href/src
	ShowImages bool                // draw image blocks (pending or ready)
	Ready      map[string][]string // resolved src -> placeholder rows to draw, all equally wide
	Failed     map[string]bool     // resolved src -> image could not be shown
}

// Image describes one image block in a rendered Document.
type Image struct {
	Src    string
	Width  int         // declared width in CSS pixels, 0 if not given
	Srcset []Candidate // alternative sources with width descriptors
	Line   int         // index in Document.Lines of the block's first line
	Height int         // number of lines the block occupies
}

// Candidate is one "url 640w" entry of an <img srcset>.
type Candidate struct {
	URL string
	W   int
}

// Document is the rendered post.
type Document struct {
	Lines  []string
	Images []Image
}

// String joins the document's lines.
func (d Document) String() string { return strings.Join(d.Lines, "\n") }

// Render parses htmlStr and renders it at o.Width.
func Render(htmlStr string, o Options) Document {
	if strings.TrimSpace(htmlStr) == "" {
		return Document{}
	}
	root, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return Document{}
	}

	r := newRenderer(o, ContentWidth(o))
	r.walkChildren(body(root))
	b := join(r.finish())
	if m := margin(o); m > 0 {
		indent := strings.Repeat(" ", m)
		for i, line := range b.lines {
			if line != "" {
				b.lines[i] = indent + line
			}
		}
	}
	return Document{Lines: b.lines, Images: b.images}
}

// ContentWidth is the width text wraps at and images must fit in: o.Width
// less the margin each side.
func ContentWidth(o Options) int {
	return max(o.Width-2*margin(o), 1)
}

// margin is o.Margin, dropped when it would leave too little room for text.
func margin(o Options) int {
	if o.Margin <= 0 || o.Width-2*o.Margin < 12 {
		return 0
	}
	return o.Margin
}

// body returns the <body> element, or n itself if there is none.
func body(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if b := body(c); b.Data == "body" {
			return b
		}
	}
	return n
}

// resolve makes ref absolute against base; it returns ref unchanged on error.
func resolve(base, ref string) string {
	ref = strings.TrimSpace(ref)
	b, err := url.Parse(base)
	if err != nil || base == "" {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return b.ResolveReference(u).String()
}

// attr returns an attribute's value with control characters removed.
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return clean(a.Val)
		}
	}
	return ""
}
