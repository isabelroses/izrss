package lib

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var (
	MainStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			Margin(0)

	ReaderStyle = lipgloss.NewStyle()
)

func TableStyle() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		Bold(true).
		Padding(0)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	s.Cell.Padding(0)

	return s
}
