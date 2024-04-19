package cmd

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/isabelroses/izrss/lib"
)

// Model is the main model for the application
type Model struct {
	help     help.Model
	filter   textinput.Model
	post     lib.Post
	context  string
	viewport viewport.Model
	keys     keyMap
	feed     lib.Feed
	feeds    lib.Feeds
	table    table.Model
	ready    bool
}

// Init sets the initial state of the model
func (m Model) Init() tea.Cmd {
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

	return Model{
		context:  "",
		feeds:    lib.Feeds{},
		feed:     lib.Feed{},
		viewport: viewport.Model{},
		table:    t,
		ready:    false,
		keys:     keys,
		help:     help.New(),
		post:     lib.Post{},
		filter:   f,
	}
}
