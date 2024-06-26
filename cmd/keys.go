// Package cmd contains all the command functions
package cmd

import (
	"log"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/isabelroses/izrss/lib"
)

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
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

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Back, k.Open},
		{k.Search},
		{k.Refresh, k.RefreshAll},
		{k.ToggleRead, k.ReadAll},
		{k.Help, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
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

// TODO: refator this so its per page and not global
func (m Model) handleKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.context == "search" {
		switch msg.String() {
		case "enter":
			m = m.loadSearchValues()

		case "ctrl+c", "esc", "/":
			m = m.loadContent(m.table.Cursor())
			m.table.Focus()
			m.filter.Blur()
		}

		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.table.SetHeight(m.viewport.Height - lipgloss.Height(m.help.View(m.keys)) - lib.MainStyle.GetBorderBottomSize())

	case key.Matches(msg, m.keys.Quit):
		err := m.feeds.WriteTracking()
		if err != nil {
			log.Fatalf("Could not write tracking data: %s", err)
		}
		return m, tea.Quit

	case key.Matches(msg, m.keys.Refresh):
		switch m.context {
		case "home":
			id := m.table.Cursor()
			feed := &m.feeds[id]
			lib.FetchURL(feed.URL, false)
			feed.Posts = lib.GetPosts(feed.URL)
			err := error(nil)
			m.feeds, err = m.feeds.ReadTracking()
			if err != nil {
				log.Fatal(err)
			}
			m = m.loadHome()

		case "content":
			feed := &m.feed
			feed.Posts = lib.GetPosts(feed.URL)
			err := error(nil)
			m.feeds, err = m.feeds.ReadTracking()
			if err != nil {
				log.Fatal(err)
			}
			m = m.loadContent(m.feed.ID)

		default:
			return m, nil
		}

	case key.Matches(msg, m.keys.RefreshAll):
		if m.context == "home" {
			m.feeds = lib.GetAllContent(lib.UserConfig.Urls, false)
			err := error(nil)
			m.feeds, err = m.feeds.ReadTracking()
			if err != nil {
				log.Fatal(err)
			}
			m = m.loadHome()
		}

	case key.Matches(msg, m.keys.Back):
		switch m.context {
		case "reader":
			m = m.loadContent(m.feed.ID)
			m.table.SetCursor(m.post.ID)
		case "content":
			m = m.loadHome()
			m.table.SetCursor(m.feed.ID)
		}
		m.viewport.SetYOffset(0)

	case key.Matches(msg, m.keys.Open):
		switch m.context {
		case "reader":
			err := lib.OpenURL(m.post.Link)
			if err != nil {
				log.Panic(err)
			}

		case "content":
			m = m.loadReader()

		default:
			m = m.loadContent(m.table.Cursor())
			m.table.SetCursor(0)
			m.viewport.SetYOffset(0)
		}

	case key.Matches(msg, m.keys.Search):
		if m.context != "search" {
			m = m.loadSearch()
		}

	case key.Matches(msg, m.keys.ToggleRead):
		switch m.context {
		case "reader":
			lib.ToggleRead(m.feeds, m.feed.ID, m.post.ID)
			m = m.loadContent(m.feed.ID)
		case "content":
			lib.ToggleRead(m.feeds, m.feed.ID, m.table.Cursor())
			m = m.loadContent(m.feed.ID)
		}
		err := m.feeds.WriteTracking()
		if err != nil {
			log.Fatalf("Could not write tracking data: %s", err)
		}

	case key.Matches(msg, m.keys.ReadAll):
		switch m.context {
		case "reader":
			// if we are in the reader view, fall back to the normal mark all as read
			lib.ToggleRead(m.feeds, m.feed.ID, m.post.ID)
		case "content":
			lib.ReadAll(m.feeds, m.feed.ID)
			m = m.loadContent(m.feed.ID)
		case "home":
			lib.ReadAll(m.feeds, m.table.Cursor())
			m = m.loadHome()
		}

		err := m.feeds.WriteTracking()
		if err != nil {
			log.Fatalf("Could not write tracking data: %s", err)
		}
	}

	return m, nil
}
