package ui

// Modified from https://github.com/charmbracelet/bubbles/blob/master/help/help.go
// Customized to pass context to the help model

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpKeyMap is a map of keybindings used to generate help
type HelpKeyMap interface {
	ShortHelp(m Model) []key.Binding
	FullHelp(m Model) [][]key.Binding
}

// HelpModel contains the state of the help view
type HelpModel struct {
	Style          lipgloss.Style
	ShortSeparator string
	FullSeparator  string
	Ellipsis       string
	Width          int
	ShowAll        bool
}

// NewHelp creates a new help view with defaults
func NewHelp(styles *Styles) HelpModel {
	return HelpModel{
		ShortSeparator: " • ",
		FullSeparator:  " • ",
		Ellipsis:       "…",
		Style:          styles.Help,
	}
}

// Update is a no-op for the help model
func (m HelpModel) Update(_ tea.Msg) (HelpModel, tea.Cmd) {
	return m, nil
}

// View renders the help view
func (hm HelpModel) View(k HelpKeyMap, m Model) string {
	if hm.ShowAll {
		return hm.FullHelpView(k.FullHelp(m))
	}
	return hm.ShortHelpView(k.ShortHelp(m))
}

// ShortHelpView renders a single line help view
func (m HelpModel) ShortHelpView(bindings []key.Binding) string {
	if len(bindings) == 0 {
		return ""
	}

	var b strings.Builder
	var totalWidth int
	separator := m.Style.Inline(true).Render(m.ShortSeparator)

	for i, kb := range bindings {
		if !kb.Enabled() {
			continue
		}

		var sep string
		if totalWidth > 0 && i < len(bindings) {
			sep = separator
		}

		str := sep +
			m.Style.Inline(true).Render(kb.Help().Key) + " " +
			m.Style.Inline(true).Render(kb.Help().Desc)

		w := lipgloss.Width(str)

		if m.Width > 0 && totalWidth+w > m.Width {
			tail := " " + m.Style.Inline(true).Render(m.Ellipsis)
			tailWidth := lipgloss.Width(tail)

			if totalWidth+tailWidth < m.Width {
				b.WriteString(tail)
			}
			break
		}

		totalWidth += w
		b.WriteString(str)
	}

	return b.String()
}

// FullHelpView renders help columns from key binding slices
func (m HelpModel) FullHelpView(groups [][]key.Binding) string {
	if len(groups) == 0 {
		return ""
	}

	var (
		out        []string
		totalWidth int
		sep        = m.Style.Render(m.FullSeparator)
		sepWidth   = lipgloss.Width(sep)
	)

	for i, group := range groups {
		if group == nil || !shouldRenderColumn(group) {
			continue
		}

		var keys, descriptions []string

		for _, kb := range group {
			if !kb.Enabled() {
				continue
			}
			keys = append(keys, kb.Help().Key)
			descriptions = append(descriptions, kb.Help().Desc)
		}

		col := lipgloss.JoinHorizontal(lipgloss.Top,
			m.Style.Render(strings.Join(keys, "\n")),
			m.Style.Render(" "),
			m.Style.Render(strings.Join(descriptions, "\n")),
		)

		totalWidth += lipgloss.Width(col)
		if m.Width > 0 && totalWidth > m.Width {
			break
		}

		out = append(out, col)

		if i < len(group)-1 {
			totalWidth += sepWidth
			if m.Width > 0 && totalWidth > m.Width {
				break
			}
			out = append(out, sep)
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, out...)
}

func shouldRenderColumn(b []key.Binding) bool {
	for _, v := range b {
		if v.Enabled() {
			return true
		}
	}
	return false
}
