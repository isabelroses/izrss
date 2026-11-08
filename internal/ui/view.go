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
		if m.page.Shown() {
			content = m.readerView()
		} else {
			content = m.main.View(m.styles, m.statusLine(statusMargin))
		}
		content = m.withOverlays(content)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "izrss"
	return v
}

func (m Model) readerView() string {
	feed, _ := m.lib.Feed(m.page.Ref().FeedURL)
	return m.page.View(feed.Name(), m.styles, m.statusLine(m.page.margin()))
}

// statusMargin is the main screen's edge gap, matching the list padding.
const statusMargin = 1

// statusLine shows key hints on the left and the feed status on the right,
// margin cells (at least one) in from each edge, on a strip of the surface
// colour across the full width.
func (m Model) statusLine(margin int) string {
	margin = max(margin, 1)
	status := statusText(m.main.Unread(), m.refresh, m.now())
	h := m.help
	h.SetWidth(max(m.width-2*margin-ansi.StringWidth(status)-2, 0))
	left := h.ShortHelpView(bindings.shortHelp(m.page.Shown(), m.main.Focus()))
	pad := strings.Repeat(" ", margin)
	gap := strings.Repeat(" ", max(m.width-2*margin-ansi.StringWidth(left)-ansi.StringWidth(status), 0))
	line := ansi.Truncate(pad+left+gap+m.styles.Hint.Render(status)+pad, m.width, "")
	line += strings.Repeat(" ", max(m.width-ansi.StringWidth(line), 0))
	// Each styled run ends in a reset, which would clear the strip too:
	// re-open it after every one.
	bar := m.styles.Bar
	return bar + strings.ReplaceAll(line, ansi.ResetStyle, ansi.ResetStyle+bar) + ansi.ResetStyle
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
		layers = append(layers, centred(m.palette.View(m.width, m.styles), m.width, m.height))
	case overlayHelp:
		layers = append(layers, centred(m.helpView(), m.width, m.height))
	}
	if t := m.toasts.view(min(60, m.width), m.styles.Toast); t != "" {
		l := lipgloss.NewLayer(t)
		layers = append(layers, l.X(max(m.width-l.Width(), 0)).Y(0).Z(2))
	}
	if len(layers) == 0 {
		return base
	}
	return lipgloss.NewCompositor(append([]*lipgloss.Layer{lipgloss.NewLayer(base)}, layers...)...).Render()
}

// centred places content horizontally centred, a quarter of the way down.
func centred(content string, width, height int) *lipgloss.Layer {
	l := lipgloss.NewLayer(content)
	return l.X(max((width-l.Width())/2, 0)).Y(max((height-l.Height())/4, 0)).Z(1)
}
