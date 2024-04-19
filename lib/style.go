package lib

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var (
	// MainStyle is the main style for the application
	MainStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cdd6f4")).
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color("#313244")).
			Padding(0, 1).
			Margin(0)

	// ReaderStyle is the style for the reader
	ReaderStyle = lipgloss.NewStyle()

	// HelpStyle is the style for the help keybinds menu
	HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8"))
)

// TableStyle returns the style for the table
func TableStyle() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		Bold(true).
		Padding(0)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#1e1e2e")).
		Background(lipgloss.Color("#74c7ec")).
		Bold(false)
	s.Cell.Padding(0)

	return s
}
