package cmd

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"

	"github.com/isabelroses/izrss/lib"
)

func loadNewTable(m model, columns []table.Column, rows []table.Row) model {
	// NOTE: clear the rows first to prevent panic
	m.table.SetRows([]table.Row{})

	m.table.SetColumns(columns)
	m.table.SetRows(rows)

	// reset the cursor and how far down the viewport is
	m.table.SetCursor(0)
	m.viewport.YPosition = 0

	return m
}

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
