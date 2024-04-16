package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/isabelroses/izrss/lib"
)

func (m model) headerView() string {
	title := lib.ReaderStyle().Render(m.post.Title)
	line := strings.Repeat("─", lib.Max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m model) footerView() string {
	info := lib.ReaderStyle().Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", lib.Max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func (m model) View() string {
	out := ""

	if !m.ready {
		out = "Initializing..."
	} else if m.context == "reader" {
		out = lipgloss.JoinVertical(
			lipgloss.Top,
			m.headerView(),
			m.viewport.View(),
			m.footerView(),
		)
	} else {
		out = lib.MainStyle().Render(m.viewport.View())
	}

	return out
}
