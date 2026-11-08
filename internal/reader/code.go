package reader

import (
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/net/html"
)

// codeLanguage reads "language-x" / "lang-x" from <pre> or its <code> child.
func codeLanguage(pre *html.Node) string {
	nodes := []*html.Node{pre}
	for c := pre.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "code" {
			nodes = append(nodes, c)
		}
	}
	for _, n := range nodes {
		for _, class := range strings.Fields(attr(n, "class")) {
			for _, p := range []string{"language-", "lang-"} {
				if lang, ok := strings.CutPrefix(class, p); ok {
					return lang
				}
			}
		}
	}
	return ""
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

// lexer picks chroma's lexer by language name, else by file name, else plain
// text, so every block is drawn in the code theme's own colours.
func lexer(lang, filename string) chroma.Lexer {
	var l chroma.Lexer
	if lang != "" {
		l = lexers.Get(lang)
	}
	if l == nil && filename != "" && !strings.ContainsAny(filename, " /") {
		l = lexers.Match(filename)
	}
	if l == nil {
		l = lexers.Fallback
	}
	return chroma.Coalesce(l)
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

// codeBox draws lines on bg with one cell of padding each side, every line
// exactly width cells wide. Lines too long are truncated, never wrapped.
func codeBox(lines []string, width int, bg color.Color) []string {
	open := style{bg: bg}.open()
	pad := 1
	if width < 3 {
		pad = 0
	}
	inner := width - 2*pad
	side := strings.Repeat(" ", pad)
	out := make([]string, len(lines))
	for i, line := range lines {
		line = ansi.Truncate(line, inner, "…")
		gap := strings.Repeat(" ", max(inner-ansi.StringWidth(line), 0))
		if open != "" {
			// Highlighted tokens end in resets, which would also clear the
			// background: re-open it after each one.
			line = strings.ReplaceAll(strings.ReplaceAll(line, "\x1b[0m", ansi.ResetStyle), ansi.ResetStyle, ansi.ResetStyle+open)
		}
		out[i] = open + side + line + gap + side + ansi.ResetStyle
	}
	return out
}
