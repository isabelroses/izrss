package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// View renders the current screen.
func (m Model) View() tea.View {
	defer logPanic()
	content := ""
	if m.ready {
		if m.screen == screenReader {
			content = m.readerView()
		} else {
			content = m.mainView()
		}
		content = m.withOverlays(content)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "izrss"
	return v
}

func (m Model) listTheme() listTheme {
	c := m.cfg.Colors
	return newListTheme(hex(c.Text), hex(c.Subtext), hex(c.Accent), hex(c.Inverttext), hex(c.Borders))
}

func (m Model) mainView() string {
	body := max(m.height-1, 0)
	lines := make([]string, 0, m.height)
	th := m.listTheme()

	switch split, left, right := m.paneWidths(); {
	case len(m.feeds) == 0:
		lines = append(lines, placeholderLines("No feeds loaded yet", m.width, body, m.styles.Hint)...)
	case split:
		l := m.feedList.view(left, m.focus == paneFeeds, th)
		r := m.postsPane(right, th)
		sep := m.styles.Border.Render("│")
		for i := range body {
			lines = append(lines, l[i]+sep+r[i])
		}
	case m.focus == paneFeeds:
		lines = append(lines, m.feedList.view(m.width, true, th)...)
	default:
		lines = append(lines, m.postsPane(m.width, th)...)
	}

	lines = append(lines, m.statusLine(statusMargin))
	return strings.Join(lines, "\n")
}

func (m Model) postsPane(width int, th listTheme) []string {
	if len(m.postList.items) == 0 {
		return placeholderLines("No posts", width, m.postList.height, m.styles.Hint)
	}
	return m.postList.view(width, m.focus == panePosts, th)
}

func (m Model) readerView() string {
	return strings.Join([]string{m.readerHeader(), m.pageView(), m.statusLine(m.pageMargin())}, "\n")
}

// statusMargin is the main screen's edge gap, matching the list padding.
const statusMargin = 1

// statusLine shows key hints on the left and the feed status on the right,
// margin cells in from each edge.
func (m Model) statusLine(margin int) string {
	status := statusText(m.unread, m.refresh, m.now())
	h := m.help
	h.SetWidth(max(m.width-2*margin-ansi.StringWidth(status)-2, 0))
	left := h.ShortHelpView(bindings.shortHelp(m.screen, m.focus))
	pad := strings.Repeat(" ", margin)
	gap := strings.Repeat(" ", max(m.width-2*margin-ansi.StringWidth(left)-ansi.StringWidth(status), 0))
	return ansi.Truncate(pad+left+gap+m.styles.Hint.Render(status)+pad, m.width, "")
}

// placeholderLines fills height lines with msg on the first one.
func placeholderLines(msg string, width, height int, st lipgloss.Style) []string {
	lines := make([]string, height)
	blank := strings.Repeat(" ", max(width, 0))
	for i := range lines {
		lines[i] = blank
	}
	if height > 0 && width > 0 {
		text := ansi.Truncate(" "+msg, width, "…")
		lines[0] = st.Render(text) + strings.Repeat(" ", max(width-ansi.StringWidth(text), 0))
	}
	return lines
}

// withOverlays draws the open overlay and toasts over the screen.
func (m Model) withOverlays(base string) string {
	var layers []*lipgloss.Layer
	switch m.overlay {
	case overlayPalette:
		layers = append(layers, centred(m.palette.view(m.feeds, m.width, m.listTheme(), m.styles), m.width, m.height))
	case overlayHelp:
		layers = append(layers, centred(m.helpView(), m.width, m.height))
	}
	if t := m.toasts.view(min(60, m.width), m.toastTheme()); t != "" {
		l := lipgloss.NewLayer(t)
		layers = append(layers, l.X(max(m.width-l.Width(), 0)).Y(0).Z(2))
	}
	if len(layers) == 0 {
		return base
	}
	return compose(base, layers...)
}

// compose stacks layers over base. Only the rows the layers cover go through
// the compositor, so a one-line toast doesn't re-parse the whole screen.
func compose(base string, layers ...*lipgloss.Layer) string {
	top, bottom := layers[0].GetY(), 0
	for _, l := range layers {
		top = min(top, l.GetY())
		bottom = max(bottom, l.GetY()+l.Height())
	}
	lines := strings.Split(base, "\n")
	top = min(top, len(lines))
	end := min(bottom, len(lines))

	band := []*lipgloss.Layer{lipgloss.NewLayer(strings.Join(lines[top:end], "\n"))}
	for _, l := range layers {
		band = append(band, l.Y(l.GetY()-top))
	}
	mid := lipgloss.NewCompositor(band...).Render()

	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:top]...)
	out = append(out, mid)
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n")
}

// centred places content horizontally centred, a quarter of the way down.
func centred(content string, width, height int) *lipgloss.Layer {
	l := lipgloss.NewLayer(content)
	return l.X(max((width-l.Width())/2, 0)).Y(max((height-l.Height())/4, 0)).Z(1)
}
