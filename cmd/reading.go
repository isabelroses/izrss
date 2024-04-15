package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/isabelroses/izrss/lib"
)

func loadReader(m model, post lib.Post) model {
	m.context = "reader"
	m.post = post
	content := lib.RenderMarkdown(post.Content)
	m.viewport.SetContent(content)
	m.viewport.YPosition = 0 // reset the viewport position

	return m
}

func (m model) headerView() string {
	title := lib.ReaderStyle().Render(m.post.Title)
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m model) footerView() string {
	info := lib.ReaderStyle().Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}
