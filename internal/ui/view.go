package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// View renders the model as a string
func (m Model) View() string {
	// Wait for window size before rendering
	if !m.ready {
		return ""
	}

	if m.context.curr == "reader" {
		return m.styles.Main.Render(
			lipgloss.JoinVertical(
				lipgloss.Top,
				fmt.Sprintf("%s - %3.f%%", m.context.post.Title, m.viewport.ScrollPercent()*100),
				m.viewport.View(),
				m.help.View(m.keys, m),
			),
		)
	}

	return m.styles.Main.Render(m.viewport.View())
}
