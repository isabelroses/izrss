// Package ui provides the terminal user interface for izrss
package ui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/isabelroses/izrss/internal/config"
)

// Styles holds the application styles
type Styles struct {
	Main lipgloss.Style
	Help lipgloss.Style
}

// NewStyles creates styles based on the configuration
func NewStyles(cfg *config.Config) *Styles {
	return &Styles{
		Main: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cfg.Colors.Text)).
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color(cfg.Colors.Borders)).
			Padding(0, 1).
			Margin(0),
		Help: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.Subtext)),
	}
}

// TableStyles returns the style for tables based on the configuration
func TableStyles(cfg *config.Config) table.Styles {
	return table.Styles{
		Header: lipgloss.NewStyle().Bold(true),
		Cell:   lipgloss.NewStyle(),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cfg.Colors.Inverttext)).
			Background(lipgloss.Color(cfg.Colors.Accent)),
	}
}
