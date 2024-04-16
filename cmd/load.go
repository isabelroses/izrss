package cmd

import (
	"github.com/charmbracelet/bubbles/table"

	"github.com/isabelroses/izrss/lib"
)

// load the home view, this conists of the list of feeds
func loadHome(m model) model {
	columns := []table.Column{
		{Title: "Title", Width: m.table.Width()},
	}

	rows := []table.Row{}
	for _, Feeds := range m.feeds {
		rows = append(rows, table.Row{Feeds.Title})
	}

	m = loadNewTable(m, columns, rows)
	m.context = "home"

	return m
}

func loadContent(m model) model {
	id := m.table.Cursor()
	feed := m.feeds[id]
	feed.ID = id

	columns := []table.Column{
		{Title: "Title", Width: m.table.Width() - 15},
		{Title: "Date", Width: 13},
	}

	rows := []table.Row{}
	for _, post := range feed.Posts {
		rows = append(rows, table.Row{post.Title, post.Date})
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
	m.viewport.SetYOffset(0)

	return m
}

func loadReader(m model) model {
	id := m.table.Cursor()
	post := m.feed.Posts[id]
	post.ID = id

	m.context = "reader"
	m.post = post
	m.viewport.YPosition = 0 // reset the viewport position

	// render the post
	content := lib.RenderMarkdown(post.Content)
	m.viewport.SetContent(content)

	return m
}
