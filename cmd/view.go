package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/isabelroses/izrss/lib"
)

func (m Model) headerView() string {
	title := lib.ReaderStyle.Render(m.context.post.Title)
	line := strings.Repeat("─", lib.Max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m Model) footerView() string {
	info := lib.ReaderStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", lib.Max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

// View renders the model as a string
func (m Model) View() string {
	out := ""

	if !m.ready {
		out = "Initializing..."
	} else if m.context.curr == "reader" {
		out = lipgloss.JoinVertical(
			lipgloss.Top,
			m.headerView(),
			m.viewport.View(),
			m.footerView(),
		)
	} else {
		out = lib.MainStyle.Render(m.viewport.View())
	}

	return out
}
