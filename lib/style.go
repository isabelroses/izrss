package lib

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func TableStyle() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderBottom(true).
		BorderBottomForeground(lipgloss.Color("240")).
		Bold(true).
		Padding(0)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	s.Cell.Padding(0)

	return s
}

func MainStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		PaddingLeft(1).
		PaddingRight(1)
}

func ReaderStyle() lipgloss.Style {
	return lipgloss.NewStyle()
}
