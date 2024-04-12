package cmd

import (
	"log"
	"os"
	"strconv"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/isabelroses/izrss/lib"
)

var feeds = lib.GetAllContent()

type model struct {
	context  string
	feeds    lib.Feeds
	posts    lib.Posts
	viewport viewport.Model
	table    table.Model
	ready    bool
}

func (m model) Init() tea.Cmd { return nil }

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return m.viewport.View()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// dynaimcally update the viewport size
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height
			m.table.SetWidth(msg.Width)
			m.table.SetHeight(msg.Height)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "ctrl+c":
			if m.context == "content" {
				m = loadHome(m)
			} else {
				return m, tea.Quit
			}
		case "enter":
			if m.context == "content" {
				id, _ := strconv.Atoi(m.table.SelectedRow()[0])
				post := m.posts.Posts[id]
				loadReader(post)
				// early return since we don't need to update the model
				return m, nil
			} else {
				id, _ := strconv.Atoi(m.table.SelectedRow()[0])
				m = loadContent(m, m.feeds[id])
			}
		}
	}

	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport.SetContent(m.table.View())
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

	for i, Feeds := range feeds {
		rows = append(rows, table.Row{strconv.Itoa(i), Feeds.Title})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Bold(false)
	t.SetStyles(s)

	m.context = "home"
	m.table = t
	m.ready = true

	return m
}

func loadContent(m model, posts lib.Posts) model {
	columns := []table.Column{
		{Title: "ID", Width: 2},
		{Title: "Title", Width: 60},
		{Title: "Date", Width: 20},
	}

	rows := []table.Row{}

	for i, post := range posts.Posts {
		rows = append(rows, table.Row{strconv.Itoa(i), post.Title, post.Date})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Bold(false)
	t.SetStyles(s)

	m.context = "content"
	m.posts = posts
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

func Run() {
	m := model{"home", feeds, lib.Posts{}, viewport.Model{}, table.Model{}, false}
	m = loadHome(m)

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
