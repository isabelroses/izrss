package cmd

import (
	"log"
	"os"
	"strconv"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/isabelroses/izrss/lib"
)

type model struct {
	help     help.Model
	post     lib.Post
	context  string
	viewport viewport.Model
	keys     keyMap
	feed     lib.Feed
	feeds    lib.Feeds
	table    table.Model
	ready    bool
}

func (m model) Init() tea.Cmd { return nil }

func (m model) View() string {
	out := ""

	if !m.ready {
		out = "Initializing..."
	} else if m.context == "reader" {
		out = lipgloss.JoinVertical(
			lipgloss.Top,
			m.headerView(),
			m.viewport.View(),
			m.footerView(),
		)
	} else {
		out = lib.MainStyle().Render(m.viewport.View())
	}

	return out
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// dynaimcally update the viewport size
		// somewhat abitary width and height to handle the borders, but they work
		width := msg.Width - 2
		height := msg.Height - 2
		if !m.ready {
			m.feeds = lib.GetAllContent()
			m = loadHome(m)
			m.viewport = viewport.New(width, height)
			m.ready = true
		} else {
			m.viewport.Width = width
			m.viewport.Height = height
		}
		m.table.SetWidth(width)
		m.table.SetHeight(height - lipgloss.Height(m.help.View(m.keys)) - 1)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			// since the help text is dynamic, we need to update the table height
			m.table.SetHeight(m.viewport.Height - lipgloss.Height(m.help.View(m.keys)) - 1)
		case key.Matches(msg, m.keys.Quit):
			switch m.context {
			case "reader":
				m = loadContent(m, m.feed)
			case "content":
				m = loadHome(m)
			default:
				return m, tea.Quit
			}

		case key.Matches(msg, m.keys.Refresh):
			if m.context == "home" {
				id, _ := strconv.Atoi(m.table.SelectedRow()[0])
				feed := &m.feeds[id]
				lib.FetchURL(feed.URL, false)
				feed.Posts = lib.GetPosts(feed.URL)
			}

		case key.Matches(msg, m.keys.Open):
			switch m.context {
			case "reader":
				lib.OpenURL(m.post.Link)
			case "content":
				id, _ := strconv.Atoi(m.table.SelectedRow()[0])
				post := m.feed.Posts[id]
				m = loadReader(m, post)
			default:
				id, _ := strconv.Atoi(m.table.SelectedRow()[0])
				m = loadContent(m, m.feeds[id])
			}
		}
	}

	// update the table, help, and viewport
	m.help, cmd = m.help.Update(msg)
	cmds = append(cmds, cmd)

	if m.context != "reader" {
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
		view := lipgloss.JoinVertical(lipgloss.Top,
			m.table.View(),
			m.help.View(m.keys),
		)
		m.viewport.SetContent(view)
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func Run() {
	m := newModel()

	if _, err := tea.NewProgram(
		m,
		tea.WithMouseCellMotion(),
	).Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
