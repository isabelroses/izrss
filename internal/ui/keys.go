package ui

import (
	"log"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/isabelroses/izrss/internal/rss"
)

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	JumpUp     key.Binding
	JumpDown   key.Binding
	Back       key.Binding
	Help       key.Binding
	Quit       key.Binding
	Open       key.Binding
	Refresh    key.Binding
	RefreshAll key.Binding
	Search     key.Binding
	ToggleRead key.Binding
	ReadAll    key.Binding
}

func (k keyMap) ShortHelp(m Model) []key.Binding {
	if m.context.curr == "reader" {
		return []key.Binding{k.Open, k.ToggleRead, k.Quit}
	}
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp(m Model) [][]key.Binding {
	switch m.context.curr {
	case "home":
		return [][]key.Binding{
			{k.Up, k.Down},
			{k.JumpUp, k.JumpDown},
			{k.Back, k.Open},
			{k.Search, k.ReadAll},
			{k.Refresh, k.RefreshAll},
			{k.Help, k.Quit},
		}
	case "content":
		return [][]key.Binding{
			{k.Up, k.Down},
			{k.JumpUp, k.JumpDown},
			{k.Back, k.Open},
			{k.Search},
			{k.Refresh, k.RefreshAll},
			{k.ToggleRead, k.ReadAll},
			{k.Help, k.Quit},
		}
	case "mixed":
		return [][]key.Binding{
			{k.Up, k.Down},
			{k.JumpUp, k.JumpDown},
			{k.Back, k.Open},
			{k.Search, k.ToggleRead},
			{k.Help, k.Quit},
		}
	default:
		return [][]key.Binding{}
	}
}

func (m Model) handleKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.context.curr {
	case "home":
		switch {
		case key.Matches(msg, m.keys.Open):
			m.loadContent(m.table.Cursor())
			m.table.SetCursor(0)
			m.viewport.SetYOffset(0)

		case key.Matches(msg, m.keys.Refresh):
			id := m.table.Cursor()
			feed := &m.context.feeds[id]
			m.fetcher.FetchURL(feed.URL, false)
			feed.Posts = m.fetcher.GetPosts(feed.URL)
			if err := m.context.feeds.ReadTracking(m.db); err != nil {
				log.Printf("error reading tracking: %v", err)
			}
			m.loadHome()
			m.table.MoveDown(id)

		case key.Matches(msg, m.keys.RefreshAll):
			m.context.feeds = m.fetcher.GetAllContent(m.cfg.Urls, false)
			if err := m.context.feeds.ReadTracking(m.db); err != nil {
				log.Printf("error reading tracking: %v", err)
			}
			m.loadHome()

		case key.Matches(msg, m.keys.ReadAll):
			rss.ReadAll(m.context.feeds, m.table.Cursor())
			m.loadHome()
			if err := m.context.feeds.WriteTracking(m.db); err != nil {
				log.Printf("error writing tracking: %v", err)
			}
		}

	case "content":
		switch {
		case key.Matches(msg, m.keys.Refresh):
			feed := &m.context.feed
			feed.Posts = m.fetcher.GetPosts(feed.URL)
			if err := m.context.feeds.ReadTracking(m.db); err != nil {
				log.Printf("error reading tracking: %v", err)
			}
			m.loadContent(m.context.feed.ID)

		case key.Matches(msg, m.keys.Back):
			m.loadHome()
			m.table.SetCursor(m.context.feed.ID)
			m.viewport.SetYOffset(0)

		case key.Matches(msg, m.keys.Open):
			m.loadReader()

		case key.Matches(msg, m.keys.ToggleRead):
			rss.ToggleRead(m.context.feeds, m.context.feed.ID, m.table.Cursor())
			m.loadContent(m.context.feed.ID)
			if err := m.context.feeds.WriteTracking(m.db); err != nil {
				log.Printf("error writing tracking: %v", err)
			}

		case key.Matches(msg, m.keys.ReadAll):
			rss.ReadAll(m.context.feeds, m.context.feed.ID)
			m.loadContent(m.context.feed.ID)
			if err := m.context.feeds.WriteTracking(m.db); err != nil {
				log.Printf("error writing tracking: %v", err)
			}
		}

	case "mixed":
		switch {
		case key.Matches(msg, m.keys.Open):
			m.loadReader()

		case key.Matches(msg, m.keys.ToggleRead):
			rss.ToggleRead(m.context.feeds, m.context.feed.ID, m.table.Cursor())
			m.loadMixed()
			if err := m.context.feeds.WriteTracking(m.db); err != nil {
				log.Printf("error writing tracking: %v", err)
			}

		case key.Matches(msg, m.keys.ReadAll):
			rss.ReadAll(m.context.feeds, m.context.feed.ID)
			m.loadMixed()
			if err := m.context.feeds.WriteTracking(m.db); err != nil {
				log.Printf("error writing tracking: %v", err)
			}
		}

	case "reader":
		switch {
		case key.Matches(msg, m.keys.Back):
			if m.context.prev == "mixed" {
				m.loadMixed()
			} else {
				m.loadContent(m.context.feed.ID)
			}
			m.table.SetCursor(m.context.post.ID)
			m.viewport.SetYOffset(0)

		case key.Matches(msg, m.keys.Open):
			if err := openURL(m.context.post.Link); err != nil {
				log.Printf("error opening URL: %v", err)
			}

		case key.Matches(msg, m.keys.ToggleRead):
			rss.ToggleRead(m.context.feeds, m.context.feed.ID, m.context.post.ID)
			m.loadContent(m.context.feed.ID)
			if err := m.context.feeds.WriteTracking(m.db); err != nil {
				log.Printf("error writing tracking: %v", err)
			}
		}

	case "search":
		switch msg.String() {
		case "enter":
			m.loadSearchValues()

		case "ctrl+c", "esc", "/":
			m.loadContent(m.table.Cursor())
			m.table.Focus()
			m.filter.Blur()
		}
	}

	// Global keys
	switch {
	case key.Matches(msg, m.keys.JumpUp):
		m.table.MoveUp(5)
	case key.Matches(msg, m.keys.JumpDown):
		m.table.MoveDown(5)

	case key.Matches(msg, m.keys.Search):
		if m.context.curr != "search" {
			m.loadSearch()
		}

	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.table.SetHeight(m.viewport.Height - lipgloss.Height(m.help.View(m.keys, m)))

	case key.Matches(msg, m.keys.Quit):
		if err := m.context.feeds.WriteTracking(m.db); err != nil {
			log.Printf("error writing tracking: %v", err)
		}
		return m, tea.Quit
	}

	return m, nil
}

var defaultKeyMap = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	JumpUp: key.NewBinding(
		key.WithKeys("shift+up", "K"),
		key.WithHelp("↑/k", "jump move up"),
	),
	JumpDown: key.NewBinding(
		key.WithKeys("shift+down", "J"),
		key.WithHelp("↓/j", "jump move down"),
	),
	Back: key.NewBinding(
		key.WithKeys("left", "h", "shift+tab"),
		key.WithHelp("←/h", "back"),
	),
	Open: key.NewBinding(
		key.WithKeys("enter", "o", "right", "l", "tab"),
		key.WithHelp("o/enter", "open"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q/esc", "quit"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	RefreshAll: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "refresh all"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	ToggleRead: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "toggle read"),
	),
	ReadAll: key.NewBinding(
		key.WithKeys("X"),
		key.WithHelp("X", "mark all as read"),
	),
}

// openURL opens the specified URL in the default browser
func openURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		if isWSL() {
			cmd = "cmd.exe"
			args = []string{"/c", "start", url}
		} else {
			cmd = "xdg-open"
			args = []string{url}
		}
	}

	return exec.Command(cmd, args...).Start()
}

func isWSL() bool {
	releaseData, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(releaseData)), "microsoft")
}
