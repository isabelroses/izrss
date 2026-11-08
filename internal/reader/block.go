package reader

import (
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/net/html"
)

// block is a rendered run of lines plus the image blocks inside it, with
// Image.Line relative to the start of lines.
type block struct {
	lines  []string
	images []Image
	tight  bool // joined without a blank line before it (nested lists)
}

// join stacks blocks with one blank line between them, shifting image lines.
func join(blocks []block) block {
	var out block
	for _, b := range blocks {
		if len(b.lines) == 0 {
			continue
		}
		if len(out.lines) > 0 && !b.tight {
			out.lines = append(out.lines, "")
		}
		out.images = appendShifted(out.images, b.images, len(out.lines))
		out.lines = append(out.lines, b.lines...)
	}
	return out
}

// renderer walks one container's children at a fixed width.
type renderer struct {
	o      Options
	codeBg color.Color // the code theme's background
	width  int
	depth  int // list nesting depth
	blocks []block
	inline inline
}

func newRenderer(o Options, width int) *renderer {
	return &renderer{o: o, codeBg: codeBackground(o.Theme.CodeTheme), width: width, inline: inline{theme: o.Theme}}
}

// sub renders n's children in a fresh renderer of the given width and options.
func (r *renderer) sub(n *html.Node, width int, o Options, depth int) block {
	s := newRenderer(o, max(width, 1))
	s.depth = depth
	s.walkChildren(n)
	return join(s.finish())
}

// finish flushes pending inline text and returns the blocks.
func (r *renderer) finish() []block {
	r.flush()
	return r.blocks
}

// flush turns accumulated inline text into a paragraph block.
func (r *renderer) flush() {
	if lines := r.inline.wrap(r.width); len(lines) > 0 {
		r.blocks = append(r.blocks, block{lines: lines})
	}
	r.inline.reset()
}

func (r *renderer) emit(b block) {
	r.flush()
	r.blocks = append(r.blocks, b)
}

func (r *renderer) walkChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		r.walk(c)
	}
}

var dropped = map[string]bool{
	"script": true, "style": true, "iframe": true, "noscript": true, "head": true,
	"template": true, "svg": true, "object": true, "embed": true, "form": true,
	"input": true, "button": true, "select": true, "textarea": true,
}

// containers are block elements rendered by simply flushing around their children.
var containers = map[string]bool{
	"p": true, "div": true, "section": true, "article": true, "header": true,
	"footer": true, "main": true, "aside": true, "nav": true, "figure": true,
	"dl": true, "dt": true, "dd": true, "address": true, "details": true, "summary": true,
}

func (r *renderer) walk(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		r.inline.text(clean(n.Data))
		return
	case html.ElementNode:
	default:
		r.walkChildren(n)
		return
	}

	switch tag := n.Data; {
	case dropped[tag]:
	case tag == "br":
		r.inline.lineBreak()
	case isHeading(tag):
		r.heading(n, int(tag[1]-'0'))
	case tag == "ul" || tag == "ol":
		r.emit(r.list(n))
	case tag == "blockquote":
		r.emit(r.blockquote(n))
	case tag == "hr":
		r.emit(block{lines: []string{r.subtle(strings.Repeat("-", min(8, r.width)))}})
	case tag == "pre":
		r.emit(r.pre(n))
	case tag == "table":
		r.emit(r.table(n))
	case tag == "img":
		if b, ok := r.image(n); ok {
			r.emit(b)
		}
	case tag == "figcaption" && isPre(nextElement(n)):
		// The code block below draws it as its title.
	case tag == "figcaption":
		o := r.o
		o.Theme.Text = r.o.Theme.Subtle
		r.emit(r.sub(n, r.width, o, r.depth))
	case tag == "summary":
		r.labelled(n, "▸ ", style{bold: true})
	case containers[tag]:
		r.flush()
		r.walkChildren(n)
		r.flush()
	default:
		r.inlineElement(n)
	}
}

// subtle renders s in the theme's subtle colour.
func (r *renderer) subtle(s string) string {
	return lipgloss.NewStyle().Foreground(r.o.Theme.Subtle).Render(s)
}

func isPre(n *html.Node) bool { return n != nil && n.Data == "pre" }

func isHeading(tag string) bool {
	return len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6'
}

// heading renders a heading as glamour's dark style does: h1 as a chip on
// the heading colour, the rest bold in it behind "##"-style markers.
func (r *renderer) heading(n *html.Node, level int) {
	r.flush()
	st := style{bold: true, fg: r.o.Theme.Heading}
	if level == 1 {
		r.inline.openChip(style{bold: true, fg: r.o.Theme.Chip, bg: r.o.Theme.Heading})
		r.walkChildren(n)
		r.inline.closeChip()
	} else {
		r.inline.push(st)
		r.inline.text(strings.Repeat("#", level) + " ")
		r.walkChildren(n)
		r.inline.pop()
	}
	r.flush()
}

// labelled renders n's children in st as their own paragraph, after a marker
// in the subtle colour (a heading's "## ", a summary's "▸ ").
func (r *renderer) labelled(n *html.Node, marker string, st style) {
	r.flush()
	r.inline.push(style{fg: r.o.Theme.Subtle})
	r.inline.text(marker)
	r.inline.pop()
	r.inline.push(st)
	r.walkChildren(n)
	r.inline.pop()
	r.flush()
}

// inlineElement applies an inline element's style to its children.
func (r *renderer) inlineElement(n *html.Node) {
	st, ok := r.inlineStyle(n)
	switch {
	case !ok:
		r.walkChildren(n)
	case st.bg != nil:
		r.inline.openChip(st)
		r.walkChildren(n)
		r.inline.closeChip()
	default:
		r.inline.push(st)
		r.walkChildren(n)
		r.inline.pop()
	}
	if n.Data == "a" && st.link != "" && showURL(attr(n, "href"), st.link, textContent(n)) {
		// As glamour does, the URL follows the link text, dimmed.
		r.inline.space = true
		r.inline.push(style{fg: r.o.Theme.Subtle, link: st.link})
		r.inline.text(st.link)
		r.inline.pop()
	}
}

// showURL reports whether a link's URL should follow its text: not for
// in-page anchors, links without text, or text that already is the URL.
func showURL(href, link, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(strings.TrimSpace(href), "#") {
		return false
	}
	bare := func(s string) string {
		for _, p := range []string{"https://", "http://", "mailto:"} {
			s = strings.TrimPrefix(s, p)
		}
		return strings.TrimSuffix(strings.TrimPrefix(s, "www."), "/")
	}
	return bare(text) != bare(link)
}

func (r *renderer) inlineStyle(n *html.Node) (style, bool) {
	switch n.Data {
	case "b", "strong":
		return style{bold: true}, true
	case "i", "em", "cite", "var":
		return style{italic: true}, true
	case "u", "ins":
		return style{underline: true}, true
	case "s", "del", "strike":
		return style{strike: true}, true
	case "code", "kbd", "samp", "tt":
		return style{fg: r.o.Theme.Code, bg: r.codeBg}, true
	case "a":
		link := resolve(r.o.BaseURL, attr(n, "href"))
		if !safeLink(link) {
			link = ""
		}
		return style{fg: r.o.Theme.Link, underline: true, link: link}, true
	}
	return style{}, false
}

// list renders <ul>/<ol> items with markers and hanging indents.
func (r *renderer) list(n *html.Node) block {
	ordered := n.Data == "ol"
	start := 1
	if v, err := strconv.Atoi(attr(n, "start")); err == nil && ordered {
		start = v
	}

	var items []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "li" {
			items = append(items, c)
		}
	}

	markerW := 2 // "• "
	if ordered {
		markerW = len(strconv.Itoa(start+len(items)-1)) + 2 // "10. "
	}
	markerStyle := lipgloss.NewStyle().Foreground(r.o.Theme.Text)

	out := block{tight: r.depth > 0}
	for i, li := range items {
		marker := "• "
		if ordered {
			num := strconv.Itoa(start+i) + ". "
			marker = strings.Repeat(" ", markerW-len(num)) + num
		}
		content := r.sub(li, r.width-markerW, r.o, r.depth+1)
		out.images = appendShifted(out.images, content.images, len(out.lines))
		for j, line := range content.lines {
			switch {
			case j == 0:
				line = markerStyle.Render(marker) + line
			case line != "":
				line = strings.Repeat(" ", markerW) + line
			}
			out.lines = append(out.lines, line)
		}
		if len(content.lines) == 0 {
			out.lines = append(out.lines, markerStyle.Render(marker))
		}
	}
	return out
}

// blockquote renders children in the quote colour behind a bar.
func (r *renderer) blockquote(n *html.Node) block {
	o := r.o
	o.Theme.Text = r.o.Theme.Quote
	inner := r.sub(n, r.width-2, o, r.depth)
	bar := lipgloss.NewStyle().Foreground(r.o.Theme.Quote).Render("│")
	out := block{images: inner.images}
	for _, line := range inner.lines {
		if line == "" {
			out.lines = append(out.lines, bar)
			continue
		}
		out.lines = append(out.lines, bar+" "+line)
	}
	return out
}

// pre renders preformatted text highlighted in a code box, truncated rather
// than wrapped, with a preceding figcaption as its title row.
func (r *renderer) pre(n *html.Node) block {
	code := strings.ReplaceAll(preText(n), "\t", "    ")
	code = strings.TrimRight(code, "\n")
	title := codeTitle(n)

	lines := highlight(code, lexer(codeLanguage(n), title, code), r.o.Theme.CodeTheme)
	if lines == nil {
		lines = strings.Split(code, "\n")
	}
	if title != "" {
		lines = append([]string{style{fg: r.o.Theme.Subtle}.open() + title + ansi.ResetStyle}, lines...)
	}
	return block{lines: codeBox(lines, r.width, r.codeBg)}
}

// preBlocks are elements a browser starts a new line for inside <pre>.
var preBlocks = map[string]bool{
	"div": true, "p": true, "li": true, "tr": true, "section": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
}

// preText extracts preformatted text, breaking lines at <br> and around block
// elements: some highlighters emit one <div> per line with no newlines.
func preText(n *html.Node) string {
	var b strings.Builder
	endsLine := func() bool { return b.Len() == 0 || strings.HasSuffix(b.String(), "\n") }
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			b.WriteString(clean(n.Data))
			return
		case n.Type == html.ElementNode && n.Data == "br":
			b.WriteByte('\n')
			return
		}
		block := n.Type == html.ElementNode && preBlocks[n.Data]
		if block && !endsLine() {
			b.WriteByte('\n')
		}
		start := b.Len()
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if block && (b.Len() == start || !endsLine()) {
			b.WriteByte('\n') // an empty block is a blank line
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}
	return b.String()
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return clean(n.Data)
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textContent(c))
	}
	return b.String()
}

// appendShifted appends images to dst, moved down by lines.
func appendShifted(dst, images []Image, by int) []Image {
	for _, img := range images {
		img.Line += by
		dst = append(dst, img)
	}
	return dst
}
