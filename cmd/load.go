package cmd

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"

	"github.com/isabelroses/izrss/lib"
)

// load the home view, this conists of the list of feeds
func (m Model) loadHome() Model {
	columns := []table.Column{
		{Title: "Unread", Width: 7},
		{Title: "Title", Width: m.table.Width() - 7},
	}

	rows := []table.Row{}
	for _, Feed := range m.feeds {
		totalUnread := strconv.Itoa(Feed.GetTotalUnreads())
		rows = append(rows, table.Row{totalUnread, Feed.Title})
	}

	m.context = "home"
	m = m.loadNewTable(columns, rows)

	return m
}

func (m Model) loadContent(id int) Model {
	feed := m.feeds[id]
	feed.ID = id

	columns := []table.Column{
		{Title: "Date", Width: 11},
		{Title: "Unread", Width: 7},
		{Title: "Title", Width: m.table.Width() - 27},
	}

	rows := []table.Row{}
	for _, post := range feed.Posts {
		unread := "x"
		if !post.Read {
			unread = "✓"
		}
		rows = append(rows, table.Row{post.Date, unread, post.Title})
	}

	m = m.loadNewTable(columns, rows)
	m.context = "content"
	m.feed = feed

	return m
}

func (m Model) loadSearch() Model {
	m.context = "search"

	m.table.Blur()

	m.filter.Focus()
	m.filter.SetValue("")

	return m
}

func (m Model) loadSearchValues() Model {
	search := m.filter.Value()

	var filteredPosts []lib.Post
	rows := []table.Row{}

	for _, feed := range m.feeds {
		for _, post := range feed.Posts {
			if strings.Contains(strings.ToLower(post.Content), strings.ToLower(search)) {
				filteredPosts = append(filteredPosts, post)
				rows = append(rows, table.Row{post.Date, post.Title})
			}
		}
	}

	columns := []table.Column{
		{Title: "Date", Width: 13},
		{Title: "Title", Width: m.table.Width() - 15},
	}

	m = m.loadNewTable(columns, rows)
	m.context = "content"
	m.feed.Posts = filteredPosts
	m.table.Focus()
	m.filter.Blur()
	m.table.SetCursor(0)

	return m
}

func (m Model) loadNewTable(columns []table.Column, rows []table.Row) Model {
	t := &m.table

	// NOTE: clear the rows first to prevent panic
	t.SetRows([]table.Row{})

	t.SetColumns(columns)
	t.SetRows(rows)

	// reset the cursor and how far down the viewport is
	m.viewport.SetYOffset(0)

	return m
}

func (m Model) loadReader() Model {
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
