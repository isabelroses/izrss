package cmd

import "github.com/charmbracelet/lipgloss"

var mainStyle = func() lipgloss.Style {
	return lipgloss.NewStyle().Padding(0, 1)
}()
