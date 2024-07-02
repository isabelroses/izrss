package cmd

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/isabelroses/izrss/lib"
)

// Model is the main model for the application
type Model struct {
	help     help.Model
	glam     *glamour.TermRenderer
	context  context
	viewport viewport.Model
	filter   textinput.Model
	table    table.Model
	ready    bool
}

// Init sets the initial state of the model
func (m Model) Init() tea.Cmd {
	lib.SetupLogger()

	return tea.Batch(
		tea.SetWindowTitle("izrss"),
	)
}

// NewModel creates a new model with sensible defaults
func NewModel() Model {
	t := table.New(table.WithFocused(true))
	t.SetStyles(lib.TableStyle())

	f := textinput.New()
	f.Prompt = "Filter: "
	f.PromptStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229"))

	h := help.New()
	h.Styles.FullKey = lib.HelpStyle
	h.Styles.FullDesc = lib.HelpStyle
	h.Styles.FullSeparator = lib.HelpStyle
	h.Styles.ShortKey = lib.HelpStyle
	h.Styles.ShortDesc = lib.HelpStyle
	h.Styles.ShortSeparator = lib.HelpStyle

	return Model{
		context:  context{},
		viewport: viewport.Model{},
		table:    t,
		ready:    false,
		help:     h,
		filter:   f,
	}
}
