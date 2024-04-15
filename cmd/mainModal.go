package cmd

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/isabelroses/izrss/lib"
)

type model struct {
	help     help.Model
	feed     lib.Feed
	context  string
	keys     keyMap
	viewport viewport.Model
	feeds    lib.Feeds
	table    table.Model
	ready    bool
}

func (m model) Init() tea.Cmd { return nil }

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	return lib.MainStyle().Render(m.viewport.View())
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
		if !m.ready {
			m.feeds = lib.GetAllContent()
			m = loadHome(m)
			m.viewport = viewport.New(msg.Width-2, msg.Height-2)
			m.ready = true
		} else {
			width := msg.Width - 2
			height := msg.Height - 2
			m.viewport.Width = width
			m.viewport.Height = height
			m.table.SetWidth(width)
			m.table.SetHeight(height)
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Quit):
			if m.context == "content" {
				m = loadHome(m)
			} else {
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
			if m.context == "content" {
				id, _ := strconv.Atoi(m.table.SelectedRow()[0])
				post := m.feed.Posts[id]
				loadReader(post)
				// early return since we don't need to update the model
				return m, nil
			} else {
				id, _ := strconv.Atoi(m.table.SelectedRow()[0])
				m = loadContent(m, m.feeds[id])
			}
		}
	}

	// update the table, help, and viewport
	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)
	m.help, cmd = m.help.Update(msg)
	cmds = append(cmds, cmd)

	helpView := m.help.View(m.keys)
	height := m.viewport.Height - strings.Count(helpView, "\n") - 21 // somewhat abitary number
	m.viewport.SetContent(m.table.View() + strings.Repeat("\n", height) + helpView)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// load the home view, this conists of the list of feeds
func loadHome(m model) model {
	columns := []table.Column{
		{Title: "ID", Width: 2},
		{Title: "Title", Width: 60},
	}

	rows := []table.Row{}

	for i, Feeds := range m.feeds {
		rows = append(rows, table.Row{strconv.Itoa(i), Feeds.Title})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	t.SetStyles(lib.TableStyle())

	m.context = "home"
	m.table = t

	return m
}

func loadContent(m model, feed lib.Feed) model {
	columns := []table.Column{
		{Title: "ID", Width: 2},
		{Title: "Title", Width: 60},
		{Title: "Date", Width: 20},
	}

	rows := []table.Row{}

	for i, post := range feed.Posts {
		rows = append(rows, table.Row{strconv.Itoa(i), post.Title, post.Date})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	t.SetStyles(lib.TableStyle())

	m.context = "content"
	m.feed = feed
	m.table = t

	return m
}

func loadReader(post lib.Post) {
	p := tea.NewProgram(
		readingModel{Post: post},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func newModel() model {
	return model{
		context:  "",
		feeds:    lib.Feeds{},
		feed:     lib.Feed{},
		viewport: viewport.Model{},
		table:    table.Model{},
		ready:    false,
		keys:     keys,
		help:     help.New(),
	}
}

func Run() {
	m := newModel()

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
