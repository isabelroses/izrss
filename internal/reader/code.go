package reader

import (
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/net/html"
)

// codeHints reads what <pre> and its <code> child say about their language.
func codeHints(pre *html.Node, filename string) langHints {
	h := langHints{Filename: filename}
	nodes := []*html.Node{pre}
	for c := pre.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "code" {
			nodes = append(nodes, c)
		}
	}
	for _, n := range nodes {
		h.Classes = append(h.Classes, attr(n, "class"))
		h.Data = append(h.Data, attr(n, "data-language"), attr(n, "data-lang"))
	}
	return h
}

// codeTitle is the text of a <figcaption> directly before pre, which blogs
// use to name the file a snippet comes from.
func codeTitle(pre *html.Node) string {
	if c := prevElement(pre); c != nil && c.Data == "figcaption" {
		return strings.Join(strings.Fields(textContent(c)), " ")
	}
	return ""
}

func prevElement(n *html.Node) *html.Node {
	return siblingElement(n.PrevSibling, func(c *html.Node) *html.Node { return c.PrevSibling })
}

func nextElement(n *html.Node) *html.Node {
	return siblingElement(n.NextSibling, func(c *html.Node) *html.Node { return c.NextSibling })
}

// siblingElement walks from c with step to the first element, skipping
// whitespace; it returns nil if non-blank text comes first.
func siblingElement(c *html.Node, step func(*html.Node) *html.Node) *html.Node {
	for ; c != nil; c = step(c) {
		if c.Type == html.ElementNode {
			return c
		}
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) != "" {
			return nil
		}
	}
	return nil
}

// highlight returns code as styled lines using chroma's lexer, style and
// terminal formatter, or nil if chroma fails.
func highlight(code string, l chroma.Lexer, styleName string) []string {
	it, err := l.Tokenise(nil, code)
	if err != nil {
		return nil
	}
	var b strings.Builder
	// styles.Get returns styles.Fallback for unknown names.
	if err := formatters.TTY16m.Format(&b, styles.Get(styleName), it); err != nil {
		return nil
	}
	// Tokens such as block comments span lines; Wrap re-opens their style on
	// each line so every line stands alone. The width never forces a break.
	out := lipgloss.Wrap(strings.TrimRight(b.String(), "\n"), math.MaxInt32, "")
	return strings.Split(out, "\n")
}

// codeBackground is the code theme's background colour, or nil if it has none.
func codeBackground(styleName string) color.Color {
	bg := styles.Get(styleName).Get(chroma.Background).Background
	if !bg.IsSet() {
		return nil
	}
	return lipgloss.Color(bg.String())
}

// codeBox draws lines on bg with a row of padding above and below and two
// cells each side (less when narrow), every line exactly width cells wide.
// Lines too long are truncated, never wrapped.
func codeBox(lines []string, width int, bg color.Color) []string {
	open := style{bg: bg}.open()
	pad := 2
	switch {
	case width < 3:
		pad = 0
	case width < 12:
		pad = 1
	}
	inner := width - 2*pad
	side := strings.Repeat(" ", pad)
	blank := open + strings.Repeat(" ", width) + ansi.ResetStyle
	out := make([]string, 0, len(lines)+2)
	out = append(out, blank)
	for _, line := range lines {
		line = ansi.Truncate(line, inner, "…")
		gap := strings.Repeat(" ", max(inner-ansi.StringWidth(line), 0))
		if open != "" {
			// Highlighted tokens end in resets, which would also clear the
			// background: re-open it after each one.
			line = strings.ReplaceAll(strings.ReplaceAll(line, "\x1b[0m", ansi.ResetStyle), ansi.ResetStyle, ansi.ResetStyle+open)
		}
		out = append(out, open+side+line+gap+side+ansi.ResetStyle)
	}
	return append(out, blank)
}
