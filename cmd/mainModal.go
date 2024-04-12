package cmd

import (
	"log"
	"os"
	"strconv"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	context string
	feeds   Feeds
	posts   Posts
	table   table.Model
}

func (m model) Init() tea.Cmd { return nil }

func (m model) View() string { return m.table.View() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {

		case "esc", "q", "ctrl+c":
			if m.context == "content" {
				return loadUrls(), nil
			} else {
				return m, tea.Quit
			}
		case "enter":
			if m.context == "content" {
				id, _ := strconv.Atoi(m.table.SelectedRow()[0])
				post := m.posts.Posts[id]
				loadReader(post)
				return m, nil
			} else {
				id, _ := strconv.Atoi(m.table.SelectedRow()[0])
				return loadContent(m, m.feeds[id]), nil
			}
		}
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func loadUrls() model {
	columns := []table.Column{
		{Title: "ID", Width: 2},
		{Title: "Title", Width: 60},
	}

	rows := []table.Row{}

	feeds := Feeds{}

	for i, Feeds := range GetAllContent() {
		rows = append(rows, table.Row{strconv.Itoa(i), Feeds.Title})
		feeds = append(feeds, Feeds)
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

	m := model{"urls", feeds, Posts{}, t}

	return m
}

func loadContent(m model, posts Posts) model {
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

func loadReader(post Post) {
	p := tea.NewProgram(
		ReadingModel{Post: post},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func Run() {
	m := loadUrls()

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
