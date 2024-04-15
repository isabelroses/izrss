package cmd

import (
	"strconv"

	"github.com/charmbracelet/bubbles/table"

	"github.com/isabelroses/izrss/lib"
)

// load the home view, this conists of the list of feeds
func loadHome(m model) model {
	columns := []table.Column{
		{Title: "ID", Width: 2},
		{Title: "Title", Width: 60},
	}

	rows := []table.Row{}
	for i, Feeds := range m.feeds {
		rows = append(rows, table.Row{strconv.Itoa(i), Feeds.Title})
	}

	m = loadNewTable(m, columns, rows)
	m.context = "home"

	return m
}

func loadContent(m model, feed lib.Feed) model {
	columns := []table.Column{
		{Title: "ID", Width: 2},
		{Title: "Title", Width: 60},
		{Title: "Date", Width: 20},
	}

	rows := []table.Row{}
	for i, post := range feed.Posts {
		rows = append(rows, table.Row{strconv.Itoa(i), post.Title, post.Date})
	}

	m = loadNewTable(m, columns, rows)
	m.context = "content"
	m.feed = feed

	return m
}

func loadNewTable(m model, columns []table.Column, rows []table.Row) model {
	t := &m.table

	// NOTE: clear the rows first to prevent panic
	t.SetRows([]table.Row{})

	t.SetColumns(columns)
	t.SetRows(rows)

	// reset the cursor and how far down the viewport is
	t.SetCursor(0)
	m.viewport.YPosition = 0

	return m
}
