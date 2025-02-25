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
	var help []key.Binding

	if m.context.curr == "reader" {
		help = []key.Binding{k.Open, k.ToggleRead, k.Quit}
	} else {
		help = []key.Binding{k.Help, k.Quit}
	}

	return help
}

func (k keyMap) FullHelp(m Model) [][]key.Binding {
	var help [][]key.Binding

	switch m.context.curr {
	case "home":
		help = [][]key.Binding{
			{k.Up, k.Down},
			{k.JumpUp, k.JumpDown},
			{k.Back, k.Open},
			{k.Search, k.ReadAll},
			{k.Refresh, k.RefreshAll},
			{k.Help, k.Quit},
		}
	case "content":
		help = [][]key.Binding{
			{k.Up, k.Down},
			{k.JumpUp, k.JumpDown},
			{k.Back, k.Open},
			{k.Search},
			{k.Refresh, k.RefreshAll},
			{k.ToggleRead, k.ReadAll},
			{k.Help, k.Quit},
		}
	case "mixed":
		help = [][]key.Binding{
			{k.Up, k.Down},
			{k.JumpUp, k.JumpDown},
			{k.Back, k.Open},
			{k.Search, k.ToggleRead},
			// {k.Refresh, k.RefreshAll},
			{k.Help, k.Quit},
		}
	case "reader":
		help = [][]key.Binding{}
	}

	return help
}

// TODO: refator this so its per page and not global
func (m Model) handleKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	// handle page specific keys
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
			lib.FetchURL(feed.URL, false)
			feed.Posts = lib.GetPosts(feed.URL)
			err := m.context.feeds.ReadTracking()
			if err != nil {
				log.Fatal(err)
			}
			m.loadHome()

		case key.Matches(msg, m.keys.RefreshAll):
			m.context.feeds = lib.GetAllContent(lib.UserConfig.Urls, false)
			err := m.context.feeds.ReadTracking()
			if err != nil {
				log.Fatal(err)
			}
			m.loadHome()

		case key.Matches(msg, m.keys.ReadAll):
			lib.ReadAll(m.context.feeds, m.table.Cursor())
			m.loadHome()
			err := m.context.feeds.WriteTracking()
			if err != nil {
				log.Fatalf("Could not write tracking data: %s", err)
			}
		}

	case "content":
		switch {
		case key.Matches(msg, m.keys.Refresh):
			feed := &m.context.feed
			feed.Posts = lib.GetPosts(feed.URL)
			err := m.context.feeds.ReadTracking()
			if err != nil {
				log.Fatal(err)
			}
			m.loadContent(m.context.feed.ID)

		case key.Matches(msg, m.keys.Back):
			m.loadHome()
			m.table.SetCursor(m.context.feed.ID)
			m.viewport.SetYOffset(0)

		case key.Matches(msg, m.keys.Open):
			m.loadReader()

		case key.Matches(msg, m.keys.ToggleRead):
			lib.ToggleRead(m.context.feeds, m.context.feed.ID, m.table.Cursor())
			m.loadContent(m.context.feed.ID)
			err := m.context.feeds.WriteTracking()
			if err != nil {
				log.Fatalf("Could not write tracking data: %s", err)
			}

		case key.Matches(msg, m.keys.ReadAll):
			lib.ReadAll(m.context.feeds, m.context.feed.ID)
			m.loadContent(m.context.feed.ID)
			err := m.context.feeds.WriteTracking()
			if err != nil {
				log.Fatalf("Could not write tracking data: %s", err)
			}
		}

	case "mixed":
		switch {
		case key.Matches(msg, m.keys.Open):
			m.loadReader()

		case key.Matches(msg, m.keys.ToggleRead):
			lib.ToggleRead(m.context.feeds, m.context.feed.ID, m.table.Cursor())
			m.loadMixed()
			err := m.context.feeds.WriteTracking()
			if err != nil {
				log.Fatalf("Could not write tracking data: %s", err)
			}

		case key.Matches(msg, m.keys.ReadAll):
			lib.ReadAll(m.context.feeds, m.context.feed.ID)
			m.loadMixed()
			err := m.context.feeds.WriteTracking()
			if err != nil {
				log.Fatalf("Could not write tracking data: %s", err)
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
			err := lib.OpenURL(m.context.post.Link)
			if err != nil {
				log.Panic(err)
			}

		case key.Matches(msg, m.keys.ToggleRead):
			lib.ToggleRead(m.context.feeds, m.context.feed.ID, m.context.post.ID)
			m.loadContent(m.context.feed.ID)
			err := m.context.feeds.WriteTracking()
			if err != nil {
				log.Fatalf("Could not write tracking data: %s", err)
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

	// handle global keys
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
		err := m.context.feeds.WriteTracking()
		if err != nil {
			log.Fatalf("Could not write tracking data: %s", err)
		}
		return m, tea.Quit
	}

	return m, nil
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
