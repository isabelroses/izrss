package ui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/reader"
	"github.com/isabelroses/izrss/internal/rss"
)

// openReader shows the post ref addresses full-screen and starts loading its
// images.
func (m *Model) openReader(ref rss.PostRef) tea.Cmd {
	p, ok := m.feeds.Post(ref)
	if !ok {
		return nil
	}
	m.open = ref
	m.post = p
	m.screen = screenReader
	// The terminal may have been resized since this post's images were fitted.
	place := rawCmd(m.images.Open(sessionKey(ref), m.imageLayout()))
	m.viewport.SetYOffset(0)
	m.renderReader()
	return tea.Batch(place, imageJobs(m.images.Fetch(m.doc.Images, m.imageLayout())), m.markReadIfScrolled())
}

func (m *Model) readerKey(msg tea.KeyPressMsg) tea.Cmd {
	k := bindings
	switch {
	case key.Matches(msg, k.Back):
		m.screen = screenMain
		return nil
	case key.Matches(msg, k.ForceQuit):
		return tea.Quit
	case key.Matches(msg, k.Toggle):
		return m.setRead(m.open, !m.read.IsRead(m.open.FeedURL, m.open.Key))
	case key.Matches(msg, k.Browser):
		return m.openLink(m.post.Link)
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return tea.Batch(cmd, m.markReadIfScrolled())
}

// markReadIfScrolled marks the open post read once it has been scrolled past
// the read threshold (short posts qualify as soon as they open).
func (m *Model) markReadIfScrolled() tea.Cmd {
	if m.viewport.ScrollPercent() < m.cfg.Reader.ReadThreshold {
		return nil
	}
	if m.read.IsRead(m.open.FeedURL, m.open.Key) {
		return nil
	}
	return m.setRead(m.open, true)
}

// renderReader renders the open post into the viewport.
func (m *Model) renderReader() {
	m.doc = reader.Render(m.post.Content, m.readerOpts())
	m.viewport.SetContent(m.doc.String())
	m.docWidth = make([]int, len(m.doc.Lines))
	for i, line := range m.doc.Lines {
		m.docWidth[i] = ansi.StringWidth(line)
	}
}

// pageView draws the viewport as viewport.View does, but from line widths
// measured once per render: viewport.View re-measures them every frame, which
// is slow for image rows.
func (m Model) pageView() string {
	vp := m.viewport
	left, right, top := vp.Style.GetPaddingLeft(), vp.Style.GetPaddingRight(), vp.Style.GetPaddingTop()
	width, height := vp.Width()-left-right, vp.Height()-top
	if width <= 0 || height <= 0 || vp.TotalLineCount() != len(m.doc.Lines) || vp.Style.GetPaddingBottom() != 0 {
		return vp.View()
	}
	first := vp.YOffset()
	visible := m.docWidth[min(first, len(m.docWidth)):min(first+height, len(m.docWidth))]
	if slices.ContainsFunc(visible, func(w int) bool { return w > width }) {
		return vp.View()
	}

	blank := strings.Repeat(" ", vp.Width())
	margin := strings.Repeat(" ", left)
	lines := make([]string, 0, top+height)
	for range top {
		lines = append(lines, blank)
	}
	for i := range height {
		if i >= len(visible) {
			lines = append(lines, blank)
			continue
		}
		lines = append(lines, margin+m.doc.Lines[first+i]+strings.Repeat(" ", width-visible[i]+right))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) readerOpts() reader.Options {
	ready, failed := m.images.Display()
	return reader.Options{
		Width:      m.textWidth(),
		Margin:     docMargin,
		Theme:      m.styles.Reader,
		BaseURL:    m.post.Link,
		ShowImages: m.images.Enabled(),
		Ready:      ready,
		Failed:     failed,
	}
}

// readerMargin is the blank columns each side of the reader page.
const readerMargin = 2

// docMargin indents the post inside the page, as glamour's document margin
// does; the header and hint bar stay at the page margin.
const docMargin = 2

// pageStyle pads the reader viewport: a blank row under the header and a
// margin each side, which narrow terminals don't get.
func pageStyle(width int) lipgloss.Style {
	side := readerMargin
	if width < 40 {
		side = 0
	}
	return lipgloss.NewStyle().Padding(1, side, 0, side)
}

// pageGap is the blank rows between the article and the hint line: one, unless
// the terminal is too short to spare it.
func pageGap(height int) int {
	if height < 8 {
		return 0
	}
	return 1
}

// pageMargin is the reader's side margin; the header and hint line align
// with the text.
func (m Model) pageMargin() int { return m.viewport.Style.GetPaddingLeft() }

// textWidth is the column the open post wraps at, inside the page margins.
func (m Model) textWidth() int {
	return readerWidth(m.cfg.Reader.Size, m.viewport.Width()-m.viewport.Style.GetHorizontalFrameSize())
}

// contentWidth is the column images are fitted to: the text width inside the
// document margin.
func (m Model) contentWidth() int {
	return reader.ContentWidth(reader.Options{Width: m.textWidth(), Margin: docMargin})
}

// readerWidth is the column the reader wraps at for the configured size.
func readerWidth(size any, width int) int {
	w := 80
	switch s := size.(type) {
	case string:
		switch s {
		case "full", "fullscreen":
			w = width
		case "most":
			w = width * 3 / 4
		}
	case int64:
		w = int(s)
	}
	return min(max(w, 20), width)
}

// readerHeader is the post title, then feed and date, with the scroll
// position on the right.
func (m Model) readerHeader() string {
	meta := []string{}
	if f, ok := m.feeds.Feed(m.open.FeedURL); ok {
		meta = append(meta, f.Name())
	}
	if m.post.Date != "" {
		meta = append(meta, m.post.Date)
	}
	margin := m.pageMargin()
	pct := fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100)
	room := max(m.width-2*margin-len(pct)-2, 0)

	title := ansi.Truncate(m.post.Title, room, "…")
	rest := ""
	if len(meta) > 0 {
		rest = ansi.Truncate(" · "+strings.Join(meta, " · "), room-ansi.StringWidth(title), "…")
	}
	pad := strings.Repeat(" ", margin)
	gap := strings.Repeat(" ", max(m.width-2*margin-ansi.StringWidth(title)-ansi.StringWidth(rest)-len(pct), 1))
	line := pad + m.styles.Header.Render(title) + m.styles.Hint.Render(rest) + gap + m.styles.Hint.Render(pct) + pad
	return ansi.Truncate(line, m.width, "")
}
