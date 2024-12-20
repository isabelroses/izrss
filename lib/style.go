package lib

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var (
	// MainStyle is the main style for the application
	MainStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(UserConfig.Colors.Text)).
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color(UserConfig.Colors.Borders)).
			Padding(0, 1).
			Margin(0)

	// HelpStyle is the style for the help keybinds menu
	HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(UserConfig.Colors.Subtext))
)

// TableStyle returns the style for the table
func TableStyle() table.Styles {
	return table.Styles{
		Header: lipgloss.NewStyle().Bold(true),
		Cell:   lipgloss.NewStyle(),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(UserConfig.Colors.Inverttext)).
			Background(lipgloss.Color(UserConfig.Colors.Accent)),
	}
}
