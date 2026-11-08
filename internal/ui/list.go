package ui

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// item is one row of a list: an optional marker, a title, and a right column.
type item struct {
	key, title, right, marker string
	strong                    bool // bold title (e.g. unread)
}

// listTheme holds the list colours and the escape sequences built from them
// once, so drawing a row is plain string concatenation.
type listTheme struct {
	accent color.Color // also frames the palette box

	plain, subFg, accFg string // foregrounds
	sel, selDim         string // selection bars: focused and unfocused
}

func newListTheme(text, subtle, accent, inverse, muted color.Color) listTheme {
	fg := func(c color.Color) string { return ansi.Style{}.ForegroundColor(c).String() }
	return listTheme{
		accent: accent,
		plain:  fg(text),
		subFg:  fg(subtle),
		accFg:  fg(accent),
		sel:    ansi.Style{}.BackgroundColor(accent).ForegroundColor(inverse).String(),
		selDim: ansi.Style{}.BackgroundColor(muted).ForegroundColor(text).String(),
	}
}

const (
	sgrBold   = "\x1b[1m"
	sgrNormal = "\x1b[22m"
)

// list is a scrolling, single-selection list that renders only what's visible.
type list struct {
	items                  []item
	cursor, offset, height int
}

// setItems replaces the rows, keeping the cursor on keepKey when present and
// otherwise clamping it.
func (l *list) setItems(items []item, keepKey string) {
	l.items = items
	if keepKey != "" {
		for i, it := range items {
			if it.key == keepKey {
				l.cursor = i
				l.scroll()
				return
			}
		}
	}
	l.cursor = min(l.cursor, max(len(items)-1, 0))
	l.scroll()
}

func (l *list) move(delta int) {
	if len(l.items) == 0 {
		return
	}
	l.cursor = min(max(l.cursor+delta, 0), len(l.items)-1)
	l.scroll()
}

func (l *list) setHeight(h int) {
	l.height = max(h, 0)
	l.scroll()
}

// scroll keeps the cursor inside the visible window.
func (l *list) scroll() {
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.height > 0 && l.cursor >= l.offset+l.height {
		l.offset = l.cursor - l.height + 1
	}
	l.offset = max(min(l.offset, len(l.items)-l.height), 0)
}

func (l list) selected() (item, bool) {
	if l.cursor < 0 || l.cursor >= len(l.items) {
		return item{}, false
	}
	return l.items[l.cursor], true
}

// view renders exactly l.height lines, each exactly width cells wide.
func (l list) view(width int, focused bool, th listTheme) []string {
	lines := make([]string, l.height)
	blank := strings.Repeat(" ", max(width, 0))
	for i := range lines {
		idx := l.offset + i
		if idx >= len(l.items) || width <= 0 {
			lines[i] = blank
			continue
		}
		lines[i] = l.row(l.items[idx], idx == l.cursor, width, focused, th)
	}
	return lines
}

// row draws one item: a cell of padding each side, the marker (if any),
// the title, and the right column flush right.
func (l list) row(it item, cursor bool, width int, focused bool, th listTheme) string {
	pad := " "
	if width < 3 {
		pad = ""
	}
	inner := width - 2*len(pad)
	marker := ""
	if it.marker != "" && inner >= 4 {
		marker = it.marker + " "
	}
	right := it.right
	titleW := inner - ansi.StringWidth(marker)
	if rw := ansi.StringWidth(right); rw > 0 && titleW-rw-1 >= 4 {
		titleW -= rw + 1
	} else {
		right = ""
	}
	title := ansi.Truncate(it.title, max(titleW, 0), "…")
	gap := strings.Repeat(" ", max(titleW-ansi.StringWidth(title), 0))
	if right != "" {
		gap += " "
	}

	open, markFg, rightFg := th.plain, th.accFg, th.subFg
	switch {
	case cursor && focused:
		open, markFg, rightFg = th.sel, "", ""
	case cursor:
		open, markFg = th.selDim, ""
	}
	if it.strong {
		title = sgrBold + title + sgrNormal
	}
	if marker != "" && markFg != "" {
		marker = markFg + marker + th.plain
	}
	var b strings.Builder
	b.Grow(len(open) + len(title) + len(gap) + len(right) + 32)
	b.WriteString(open)
	b.WriteString(pad)
	b.WriteString(marker)
	b.WriteString(title)
	b.WriteString(gap)
	b.WriteString(rightFg)
	b.WriteString(right)
	b.WriteString(pad)
	b.WriteString(ansi.ResetStyle)
	return b.String()
}
