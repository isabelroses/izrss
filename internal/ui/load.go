package ui

import (
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/mattn/go-runewidth"

	"github.com/isabelroses/izrss/internal/rss"
)

// boldUnread bolds s with an explicit bold-off (SGR 22) instead of a full reset
// so the table's selected-row highlight survives, and pre-truncates so the
// table's ANSI-unaware truncation can't cut an escape sequence.
func boldUnread(s string, width int) string {
	const on, off = "\x1b[1m", "\x1b[22m"
	overhead := runewidth.StringWidth(on) + runewidth.StringWidth(off)
	if width > overhead && runewidth.StringWidth(s)+overhead > width {
		s = runewidth.Truncate(s, width-overhead, "…")
	}
	return on + s + off
}

// loadHome loads the home view with the list of feeds
func (m *Model) loadHome() {
	titleWidth := m.table.Width() - 10
	columns := []table.Column{
		{Title: "Unread", Width: 10},
		{Title: "Title", Width: titleWidth},
	}

	rows := make([]table.Row, 0, len(m.context.feeds))
	for _, feed := range m.context.feeds {
		unread := feed.GetTotalUnreads()
		fraction := fmt.Sprintf("%d/%d", unread, len(feed.Posts))

		title := feed.Title
		if unread > 0 {
			title = boldUnread(title, titleWidth)
		}

		rows = append(rows, table.Row{fraction, title})
	}

	m.swapPage("home")
	m.loadNewTable(columns, rows)
}

func (m *Model) postColumns() []table.Column {
	return []table.Column{
		{Title: "", Width: 2},
		{Title: "Date", Width: 15},
		{Title: "Title", Width: m.table.Width() - 17},
	}
}

func (m *Model) loadMixed() {
	total := 0
	for _, feed := range m.context.feeds {
		total += len(feed.Posts)
	}

	posts := make([]rss.Post, 0, total)
	for _, feed := range m.context.feeds {
		posts = append(posts, feed.Posts...)
	}

	rss.SortPosts(posts)

	rows := make([]table.Row, len(posts))
	for i, post := range posts {
		rows[i] = table.Row{rss.ReadSymbol(post.Read), post.Date, post.Title}
	}

	m.context.feed = rss.Feed{Title: "Mixed", Posts: posts, ID: 0, URL: ""}

	m.loadNewTable(m.postColumns(), rows)
	m.swapPage("mixed")
}

func (m *Model) loadContent(id int) {
	feed := m.context.feeds[id]
	feed.ID = id

	rows := make([]table.Row, 0, len(feed.Posts))
	for _, post := range feed.Posts {
		rows = append(rows, table.Row{rss.ReadSymbol(post.Read), post.Date, post.Title})
	}

	m.loadNewTable(m.postColumns(), rows)
	m.swapPage("content")
	m.context.feed = feed
}

func (m *Model) loadSearch() {
	m.swapPage("search")
	m.table.Blur()
	m.filter.Focus()
	m.filter.SetValue("")
}

func (m *Model) loadSearchValues() {
	search := m.filter.Value()

	var filteredPosts []rss.Post
	var rows []table.Row

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

	m.loadNewTable(columns, rows)
	m.swapPage("content")
	m.context.feed.Posts = filteredPosts
	m.table.Focus()
	m.filter.Blur()
	m.table.SetCursor(0)
}

func (m *Model) loadNewTable(columns []table.Column, rows []table.Row) {
	cursor := m.table.Cursor()

	// Clear the rows before swapping columns so a row can't be rendered against a
	// different column count (which panics). bubbles >= v1 clamps the cursor to
	// the row count on SetRows, so clearing detaches it; reattach + clamp after.
	m.table.SetRows(nil)
	m.table.SetColumns(columns)
	m.table.SetRows(rows)
	m.table.SetCursor(cursor)
}

func (m *Model) loadReader() {
	id := m.table.Cursor()
	post := m.context.feed.Posts[id]
	post.ID = id

	m.swapPage("reader")
	m.context.post = post
	m.viewport.YPosition = 0

	// Render the post
	fromMd, err := htmlToMarkdown.ConvertString(post.Content)
	if err != nil {
		log.Printf("could not convert html to markdown: %v", err)
		fromMd = post.Content
	}

	out, err := m.glam.Render(fromMd)
	if err != nil {
		log.Printf("could not render markdown: %v", err)
		out = fromMd
	}

	m.viewport.SetContent(out)
	m.viewport.Height = m.viewport.Height - 2
}
