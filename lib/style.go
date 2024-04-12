package lib

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func TableStyle() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderBottom(true).
		BorderForeground(lipgloss.Color("240")).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	return s
}

func MainStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))
}

func ReaderStyle() lipgloss.Style {
	return lipgloss.NewStyle()
}
