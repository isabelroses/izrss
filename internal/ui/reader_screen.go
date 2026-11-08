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

// openReader shows the post at loc full-screen and starts loading its images.
func (m *Model) openReader(loc rss.Loc) tea.Cmd {
	f := m.feeds[loc.Feed]
	p := f.Posts[loc.Post]
	m.open = openRef{feedURL: f.URL, key: p.Key()}
	m.post = p
	m.screen = screenReader
	if key := postKey(p.UUID, p.Link, p.Title); key != m.imgPost {
		m.imgPost = key
		m.imgs = map[string]*postImage{}
		m.imgToasted = false
	}
	// The terminal may have been resized since this post's images were fitted.
	place := m.resizeImages()
	m.viewport.SetYOffset(0)
	m.renderReader()
	return tea.Batch(place, m.loadImages(m.doc), m.markReadIfScrolled())
}

// openLoc finds the open post in the current feeds.
func (m Model) openLoc() (rss.Loc, bool) {
	return m.feeds.Locate(m.open.feedURL, m.open.key)
}

// reopen refreshes the reader after the feeds changed underneath it.
func (m *Model) reopen() {
	if loc, ok := m.openLoc(); ok {
		m.post = m.feeds[loc.Feed].Posts[loc.Post]
		m.rerenderReader()
	}
}

func (m *Model) readerKey(msg tea.KeyPressMsg) tea.Cmd {
	k := bindings
	switch {
	case key.Matches(msg, k.Back):
		m.screen = screenMain
		return nil
	case key.Matches(msg, k.ForceQuit):
		return m.quit()
	case key.Matches(msg, k.Toggle):
		if loc, ok := m.openLoc(); ok {
			return m.toggleRead(loc)
		}
		return nil
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
	loc, ok := m.openLoc()
	if !ok || m.feeds[loc.Feed].Posts[loc.Post].Read {
		return nil
	}
	return m.setRead(loc, true)
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
// measured once per render; viewport.View re-measures them every frame, which is slow for image rows.
func (m Model) pageView() string {
	vp := m.viewport
	left, right, top := vp.Style.GetPaddingLeft(), vp.Style.GetPaddingRight(), vp.Style.GetPaddingTop()
	width, height := vp.Width()-left-right, vp.Height()-top
	if width <= 0 || height <= 0 || vp.TotalLineCount() != len(m.doc.Lines) || vp.Style.GetPaddingBottom() != 0 {
		return vp.View()
	}
	first := vp.YOffset()
	visible := m.docWidth[min(first, len(m.docWidth)):min(first+height, len(m.docWidth))]
	if slices.Max(append([]int{0}, visible...)) > width {
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
	ready, failed := m.readyRows()
	return reader.Options{
		Width:      m.textWidth(),
		Theme:      readerTheme(m.cfg),
		BaseURL:    m.post.Link,
		ShowImages: m.imgOn,
		Ready:      ready,
		Failed:     failed,
	}
}

// readerMargin is the blank columns each side of the reader page.
const readerMargin = 2

// pageStyle pads the reader viewport: a blank row under the header and a
// margin each side, which narrow terminals don't get.
func pageStyle(width int) lipgloss.Style {
	side := readerMargin
	if width < 40 {
		side = 0
	}
	return lipgloss.NewStyle().Padding(1, side, 0, side)
}

// pageMargin is the reader's side margin; the header and hint line align
// with the text.
func (m Model) pageMargin() int { return m.viewport.Style.GetPaddingLeft() }

// textWidth is the column the open post wraps at, inside the page margins.
func (m Model) textWidth() int {
	return readerWidth(m.cfg.Reader.Size, m.viewport.Width()-m.viewport.Style.GetHorizontalFrameSize())
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
	if loc, ok := m.openLoc(); ok && m.feeds[loc.Feed].Title != "" {
		meta = append(meta, m.feeds[loc.Feed].Title)
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
