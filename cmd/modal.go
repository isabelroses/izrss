package cmd

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/isabelroses/izrss/lib"
)

type model struct {
	help     help.Model
	post     lib.Post
	context  string
	viewport viewport.Model
	keys     keyMap
	feed     lib.Feed
	feeds    lib.Feeds
	table    table.Model
	ready    bool
}

func (m model) Init() tea.Cmd { return nil }

func newModel() model {
	t := table.New(table.WithFocused(true))
	t.SetStyles(lib.TableStyle())

	return model{
		context:  "",
		feeds:    lib.Feeds{},
		feed:     lib.Feed{},
		viewport: viewport.Model{},
		table:    t,
		ready:    false,
		keys:     keys,
		help:     help.New(),
		post:     lib.Post{},
	}
}
