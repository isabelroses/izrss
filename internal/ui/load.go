package ui

import (
	"bytes"
	"encoding/gob"
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

	// If using cache, batch load all parsed feeds first for speed
	if preferCache {
		return m.loadFeedsFromCacheCmd(urls)
	}

	// For fresh fetches, load one at a time with progress updates
	return m.loadFeedsFreshCmd(urls)
}

// loadFeedsFromCacheCmd loads all feeds from cache in one batch
func (m *Model) loadFeedsFromCacheCmd(urls []string) tea.Cmd {
	return func() tea.Msg {
		// Batch load all parsed feeds in one query
		cachedFeeds, err := m.db.LoadAllParsedFeeds()
		if err != nil {
			log.Printf("could not load parsed feeds cache: %v", err)
			cachedFeeds = make(map[string][]byte)
		}

		feeds := make(rss.Feeds, 0, len(urls))
		for _, url := range urls {
			if cachedData, exists := cachedFeeds[url]; exists {
				var feed rss.Feed
				decoder := gob.NewDecoder(bytes.NewReader(cachedData))
				if err := decoder.Decode(&feed); err == nil {
					feeds = append(feeds, feed)
					continue
				}
			}
			// Fallback to individual fetch if not in cache
			feeds = append(feeds, m.fetcher.GetContentForURL(url, true))
		}

		return BatchFeedsLoadedMsg{Feeds: feeds}
	}
}

// loadFeedsFreshCmd loads feeds one at a time for fresh fetches
func (m *Model) loadFeedsFreshCmd(urls []string) tea.Cmd {
	// Create commands for each feed
	cmds := make([]tea.Cmd, 0, len(urls)+1)
	for _, url := range urls {
		u := url // capture
		cmds = append(cmds, func() tea.Msg {
			feed := m.fetcher.GetContentForURL(u, false)
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

// postColumns returns the standard columns for post lists
func (m *Model) postColumns() []table.Column {
	return []table.Column{
		{Title: "", Width: 2},
		{Title: "Date", Width: 15},
		{Title: "Title", Width: m.table.Width() - 17},
	}
}

func (m *Model) loadContent(id int) {
	feed := m.context.feeds[id]
	feed.ID = id

	columns := m.postColumns()

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
