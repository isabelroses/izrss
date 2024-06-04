package cmd

import (
	"log"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/isabelroses/izrss/lib"
)

// Update will regnerate the model on each run
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) Model {
	framew, frameh := lib.MainStyle.GetFrameSize()

	height := msg.Height - frameh
	width := msg.Width - framew

	m.table.SetWidth(width)
	m.table.SetHeight(height - lipgloss.Height(m.help.View(m.keys)) - lib.MainStyle.GetBorderBottomSize())

	if !m.ready {
		m.feeds = lib.GetAllContent(lib.UserConfig.Urls, lib.CheckCache())
		m.viewport = viewport.New(width, height)

		err := error(nil)
		m.feeds, err = m.feeds.ReadTracking()
		if err != nil {
			log.Fatalf("could not read tracking file: %v", err)
		}

		m.glam, _ = glamour.NewTermRenderer(
			glamour.WithEnvironmentConfig(),
			glamour.WithWordWrap(width),
		)

		m = m.loadHome()

		m.ready = true
	} else {
		m.viewport.Width = width
		m.viewport.Height = height
	}

	return m
}

func (m Model) updateViewport(msg tea.Msg) (Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	m.help, cmd = m.help.Update(msg)
	cmds = append(cmds, cmd)
	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)

	if m.context != "reader" && m.context != "search" {
		view := lipgloss.JoinVertical(
			lipgloss.Top,
			m.table.View(),
			m.help.View(m.keys),
		)
		m.viewport.SetContent(view)
	} else if m.context == "search" {
		m.filter, cmd = m.filter.Update(msg)
		cmds = append(cmds, cmd)

		view := lipgloss.JoinVertical(
			lipgloss.Top,
			m.filter.View(),
			m.table.View(),
			m.help.View(m.keys),
		)

		m.viewport.SetContent(view)
	}

	if m.context == "reader" && m.viewport.ScrollPercent() >= lib.UserConfig.ReadThreshold {
		lib.MarkRead(m.feeds, m.feed.ID, m.post.ID)
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
