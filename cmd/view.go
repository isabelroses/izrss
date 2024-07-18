package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/isabelroses/izrss/lib"
)

// View renders the model as a string
func (m Model) View() string {
	out := ""

	if !m.ready {
		out = "Initializing..."
	} else if m.context.curr == "reader" {
		out = lib.MainStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Top,
				fmt.Sprintf("%s - %3.f%%", m.context.post.Title, m.viewport.ScrollPercent()*100),
				m.viewport.View(),
				m.help.View(m.keys, m),
			),
		)
	} else {
		out = lib.MainStyle.Render(m.viewport.View())
	}

	return out
}
