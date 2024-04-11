package cmd

import (
	"log"
	"os"
	"strconv"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	posts []Post
	table table.Model
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			id, _ := strconv.Atoi(m.table.SelectedRow()[0])
			post := m.posts[id]
			loadReader(post)
			return m, nil
		}
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return m.table.View()
}

func Run() {
	columns := []table.Column{
		{Title: "ID", Width: 2},
		{Title: "Title", Width: 60},
		{Title: "Date", Width: 20},
	}

	rows := []table.Row{}

	posts := []Post{}

	count := 0
	for _, Posts := range GetAllContent() {
		for _, post := range Posts.Posts {
			rows = append(rows, table.Row{strconv.Itoa(count), post.Title, post.Date})
			posts = append(posts, post)
			count++
		}
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

	m := model{posts, t}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
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
