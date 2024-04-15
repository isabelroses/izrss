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
	"github.com/charmbracelet/lipgloss"

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
		return "Initializing..."
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
		m.table.SetHeight(height - strings.Count(m.help.View(m.keys), "\n") - 2)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			// since the help text is dynamic, we need to update the table height
			m.table.SetHeight(m.viewport.Height - strings.Count(m.help.View(m.keys), "\n") - 2)
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

	view := lipgloss.JoinVertical(lipgloss.Top,
		m.table.View(),
		m.help.View(m.keys),
	)
	m.viewport.SetContent(view)

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

	// NOTE: clear the rows first to prevent panic
	m.table.SetRows([]table.Row{})
	m.table.SetColumns(columns)
	m.table.SetRows(rows)
	m.table.SetCursor(0) // reset the selected row to 0

	m.context = "home"

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

	// NOTE: clear the rows first to prevent panic
	m.table.SetRows([]table.Row{})
	m.table.SetColumns(columns)
	m.table.SetRows(rows)
	m.table.SetCursor(0) // reset the selected row to 0

	m.context = "content"
	m.feed = feed

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
	t := table.New(table.WithFocused(true))
	t.SetStyles(lib.TableStyle())

	return model{
		context:  "",
		feeds:    lib.Feeds{},
		feed:     lib.Feed{},
		viewport: viewport.Model{},
		table:    t,
		ready:    false,
		keys:     keys,
		help:     help.New(),
	}
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
