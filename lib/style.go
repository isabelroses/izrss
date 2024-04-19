package lib

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var (
	// MainStyle is the main style for the application
	MainStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			Margin(0)

	// ReaderStyle is the style for the reader
	ReaderStyle = lipgloss.NewStyle()
)

// TableStyle returns the style for the table
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
