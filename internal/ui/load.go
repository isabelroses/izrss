package ui

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/isabelroses/izrss/internal/rss"
)

// loadFeedsCmd creates a command that loads all feeds progressively
func (m *Model) loadFeedsCmd() tea.Cmd {
	preferCache := m.fetcher.CheckCache()
	urls := m.cfg.Urls

	// Create commands for each feed
	cmds := make([]tea.Cmd, 0, len(urls)+1)
	for _, url := range urls {
		u := url // capture
		cmds = append(cmds, func() tea.Msg {
			feed := m.fetcher.GetContentForURL(u, preferCache)
			return FeedLoadedMsg{Feed: feed}
		})
	}

	// Add final command to signal completion
	cmds = append(cmds, func() tea.Msg {
		return AllFeedsLoadedMsg{}
	})

	return tea.Sequence(cmds...)
}

// loadHome loads the home view with the list of feeds
func (m *Model) loadHome() {
	columns := []table.Column{
		{Title: "Unread", Width: 10},
		{Title: "Title", Width: m.table.Width() - 10},
	}

	rows := make([]table.Row, 0, len(m.context.feeds)+1)
	for _, feed := range m.context.feeds {
		totalUnread := strconv.Itoa(feed.GetTotalUnreads())
		fraction := fmt.Sprintf("%s/%d", totalUnread, len(feed.Posts))
		rows = append(rows, table.Row{fraction, feed.Title})
	}

	// Show loading indicator if still loading
	if m.loading && m.loadedCount < m.totalCount {
		rows = append(rows, table.Row{"", fmt.Sprintf("Loading... (%d/%d)", m.loadedCount, m.totalCount)})
	}

	m.swapPage("home")
	m.loadNewTable(columns, rows)
}

func (m *Model) loadMixed() {
	columns := []table.Column{
		{Title: "", Width: 2},
		{Title: "Date", Width: 15},
		{Title: "Title", Width: m.table.Width() - 17},
	}

	// Pre-calculate total size to avoid reallocation
	totalPosts := 0
	for _, feed := range m.context.feeds {
		totalPosts += len(feed.Posts)
	}

	posts := make([]rss.Post, 0, totalPosts)
	for _, feed := range m.context.feeds {
		posts = append(posts, feed.Posts...)
	}

	rss.SortPosts(posts, m.cfg.DateFormat)

	rows := make([]table.Row, len(posts))
	for i, post := range posts {
		read := rss.ReadSymbol(post.Read)
		rows[i] = table.Row{read, post.Date, post.Title}
	}

	m.context.feed = rss.Feed{Title: "Mixed", Posts: posts, ID: 0, URL: ""}

	m.loadNewTable(columns, rows)
	m.swapPage("mixed")
}

func (m *Model) loadContent(id int) {
	feed := m.context.feeds[id]
	feed.ID = id

	columns := []table.Column{
		{Title: "", Width: 2},
		{Title: "Date", Width: 15},
		{Title: "Title", Width: m.table.Width() - 17},
	}

	rows := make([]table.Row, 0, len(feed.Posts))
	for _, post := range feed.Posts {
		readsym := rss.ReadSymbol(post.Read)
		rows = append(rows, table.Row{readsym, post.Date, post.Title})
	}

	m.loadNewTable(columns, rows)
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
	// Clear rows first to prevent panic
	m.table.SetRows([]table.Row{})
	m.table.SetColumns(columns)
	m.table.SetRows(rows)
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
