package cmd

import (
	"log"
	"os"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/isabelroses/izrss/lib"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m = m.handleWindowSize(msg)
	case tea.KeyMsg:
		m, cmd = m.handleKeys(msg)
		cmds = append(cmds, cmd)
	}

	m, cmd = m.updateViewport(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) handleWindowSize(msg tea.WindowSizeMsg) model {
	width := msg.Width - 4   // account for padding and borders
	height := msg.Height - 2 // account for borders
	m.table.SetWidth(width)
	m.table.SetHeight(height - lipgloss.Height(m.help.View(m.keys)) - 1) // account for bottom border only
	if !m.ready {
		m.feeds = lib.GetAllContent(true)
		m.viewport = viewport.New(width, height)
		m = loadHome(m)
		m.ready = true
	} else {
		m.viewport.Width = width
		m.viewport.Height = height
	}

	return m
}

func (m model) updateViewport(msg tea.Msg) (model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	m.help, cmd = m.help.Update(msg)
	cmds = append(cmds, cmd)
	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)

	if m.context != "reader" {
		view := lipgloss.JoinVertical(
			lipgloss.Top,
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
	if _, err := tea.NewProgram(
		newModel(),
		tea.WithMouseCellMotion(),
	).Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
