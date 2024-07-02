package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"

	"github.com/isabelroses/izrss/lib"
)

// load the home view, this conists of the list of feeds
func (m Model) loadHome() Model {
	columns := []table.Column{
		{Title: "Unread", Width: 10},
		{Title: "Title", Width: m.table.Width() - 10},
	}

	rows := []table.Row{}
	for _, Feed := range m.context.feeds {
		totalUnread := strconv.Itoa(Feed.GetTotalUnreads())
		fraction := fmt.Sprintf("%s/%d", totalUnread, len(Feed.Posts))
		rows = append(rows, table.Row{fraction, Feed.Title})
	}

	m.context.curr = "home"
	m = m.loadNewTable(columns, rows)

	return m
}

func (m Model) loadMixed() Model {
	columns := []table.Column{
		{Title: "Date", Width: 15},
		{Title: "Read", Width: 10},
		{Title: "Title", Width: m.table.Width() - 25},
	}

	posts := []lib.Post{}
	for _, feed := range m.context.feeds {
		posts = append(posts, feed.Posts...)
	}

	rows := []table.Row{}
	for _, post := range posts {
		read := lib.ReadSymbol(post.Read)
		rows = append(rows, table.Row{post.Date, read, post.Title})
	}

	m.context.feed.Posts = posts

	m = m.loadNewTable(columns, rows)
	m.swapPage("mixed")

	return m
}

func (m Model) loadContent(id int) Model {
	feed := m.context.feeds[id]
	feed.ID = id

	columns := []table.Column{
		{Title: "", Width: 10},
		{Title: "Date", Width: 15},
		{Title: "Title", Width: m.table.Width() - 25},
	}

	rows := []table.Row{}
	for _, post := range feed.Posts {
		readsym := lib.ReadSymbol(post.Read)
		rows = append(rows, table.Row{readsym, post.Date, post.Title})
	}

	m = m.loadNewTable(columns, rows)
	m.swapPage("content")
	m.context.feed = feed

	return m
}

func (m Model) loadSearch() Model {
	m.swapPage("search")

	m.table.Blur()

	m.filter.Focus()
	m.filter.SetValue("")

	return m
}

func (m Model) loadSearchValues() Model {
	search := m.filter.Value()

	var filteredPosts []lib.Post
	rows := []table.Row{}

	for _, feed := range m.context.feeds {
		for _, post := range feed.Posts {
			if strings.Contains(strings.ToLower(post.Content), strings.ToLower(search)) {
				filteredPosts = append(filteredPosts, post)
				rows = append(rows, table.Row{post.Date, post.Title})
			}
		}
	}

	columns := []table.Column{
		{Title: "Date", Width: 15},
		{Title: "Title", Width: m.table.Width() - 15},
	}

	m = m.loadNewTable(columns, rows)
	m.swapPage("content")
	m.context.feed.Posts = filteredPosts
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

	return m
}
