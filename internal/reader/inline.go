package reader

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// style is one layer of inline formatting; layers merge as the stack grows.
type style struct {
	bold, italic, underline, strike bool
	fg, bg                          color.Color
	link                            string
}

func (s style) merge(top style) style {
	s.bold = s.bold || top.bold
	s.italic = s.italic || top.italic
	s.underline = s.underline || top.underline
	s.strike = s.strike || top.strike
	if top.fg != nil {
		s.fg = top.fg
	}
	if top.bg != nil {
		s.bg = top.bg
	}
	if top.link != "" {
		s.link = top.link
	}
	return s
}

// piece is a run of text sharing one style. A piece with text "\n" is a
// forced line break.
type piece struct {
	text string
	st   style
}

// inline accumulates a paragraph's text with whitespace collapsed.
type inline struct {
	theme  Theme
	stack  []style
	pieces []piece
	space  bool // a collapsed space is pending before the next word
}

func (in *inline) current() style {
	s := style{fg: in.theme.Text}
	for _, l := range in.stack {
		s = s.merge(l)
	}
	return s
}

func (in *inline) push(s style) { in.stack = append(in.stack, s) }

func (in *inline) pop() {
	if len(in.stack) > 0 {
		in.stack = in.stack[:len(in.stack)-1]
	}
}

func (in *inline) reset() {
	in.pieces = in.pieces[:0]
	in.space = false
}

// text appends HTML text, collapsing whitespace runs to single spaces.
func (in *inline) text(s string) {
	st := in.current()
	start := -1
	for i, r := range s {
		if !isSpace(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			in.addWord(s[start:i], st)
			start = -1
		}
		in.space = true
	}
	if start >= 0 {
		in.addWord(s[start:], st)
	}
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
}

func (in *inline) addWord(word string, st style) {
	in.writeSpace(st)
	in.pieces = append(in.pieces, piece{text: word, st: st})
}

// writeSpace emits the pending collapsed space in st, unless the paragraph
// is empty or the space would start a line.
func (in *inline) writeSpace(st style) {
	if in.space && len(in.pieces) > 0 && in.pieces[len(in.pieces)-1].text != "\n" {
		in.pieces = append(in.pieces, piece{text: " ", st: st})
	}
	in.space = false
}

// openChip starts a run on a background (inline code), padded with a
// non-breaking space so wrapping can't split the padding from its text.
func (in *inline) openChip(st style) {
	in.writeSpace(in.current())
	in.push(st)
	in.pieces = append(in.pieces, piece{text: "\u00a0", st: in.current()})
}

// closeChip pads and ends the chip; trailing whitespace inside it stays
// pending, outside the background.
func (in *inline) closeChip() {
	space := in.space
	in.pieces = append(in.pieces, piece{text: "\u00a0", st: in.current()})
	in.pop()
	in.space = space
}

func (in *inline) lineBreak() {
	in.pieces = append(in.pieces, piece{text: "\n"})
	in.space = false
}

// open returns the escapes that start s; close ends them.
func (s style) open() string {
	st := ansi.Style{}
	if s.bold {
		st = st.Bold()
	}
	if s.italic {
		st = st.Italic(true)
	}
	if s.underline {
		st = st.Underline(true)
	}
	if s.strike {
		st = st.Strikethrough(true)
	}
	if s.fg != nil {
		st = st.ForegroundColor(s.fg)
	}
	if s.bg != nil {
		st = st.BackgroundColor(s.bg)
	}
	out := ""
	if s.link != "" {
		out = ansi.SetHyperlink(s.link)
	}
	if len(st) > 0 {
		out += st.String()
	}
	return out
}

func (s style) close() string {
	out := ansi.ResetStyle
	if s.link != "" {
		out += ansi.ResetHyperlink()
	}
	return out
}

// styled renders the paragraph as one styled string, "\n" at forced breaks.
func (in *inline) styled() string {
	var b strings.Builder
	for i := 0; i < len(in.pieces); {
		p := in.pieces[i]
		if p.text == "\n" {
			b.WriteByte('\n')
			i++
			continue
		}
		j := i + 1
		for j < len(in.pieces) && in.pieces[j].text != "\n" && in.pieces[j].st == p.st {
			j++
		}
		b.WriteString(p.st.open())
		for _, q := range in.pieces[i:j] {
			b.WriteString(q.text)
		}
		b.WriteString(p.st.close())
		i = j
	}
	return b.String()
}

// wrap wraps the paragraph to width with lipgloss.Wrap, which re-applies
// styles and links on every line so each line is self-contained.
func (in *inline) wrap(width int) []string {
	s := in.styled()
	if strings.TrimSpace(ansi.Strip(s)) == "" {
		return nil
	}
	lines := strings.Split(lipgloss.Wrap(s, width, ""), "\n")
	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
