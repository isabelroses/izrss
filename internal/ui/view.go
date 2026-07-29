package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View renders the model as a tea.View
func (m Model) View() tea.View {
	var content string
	switch {
	case !m.ready:
		content = "Initializing..."
	case m.context.curr == "reader":
		content = m.styles.Main.Render(
			lipgloss.JoinVertical(
				lipgloss.Top,
				fmt.Sprintf("%s - %3.f%%", m.context.post.Title, m.viewport.ScrollPercent()*100),
				m.viewport.View(),
				m.help.View(m.keys, m),
			),
		)
	default:
		content = m.styles.Main.Render(m.viewport.View())
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "izrss"
	return v
}
