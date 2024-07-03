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
	m.table.SetHeight(height - lipgloss.Height(m.help.View(m.keys, m)) - lib.MainStyle.GetBorderBottomSize())

	if !m.ready {
		m.context.feeds = lib.GetAllContent(lib.UserConfig.Urls, lib.CheckCache())
		m.viewport = viewport.New(width, height)

		err := error(nil)
		m.context.feeds, err = m.context.feeds.ReadTracking()
		if err != nil {
			log.Fatalf("could not read tracking file: %v", err)
		}

		var glamWidth glamour.TermRendererOption
		switch lib.UserConfig.Reader.Size.(type) {
		case string:
			switch lib.UserConfig.Reader.Size {
			case "full", "fullscreen":
				glamWidth = glamour.WithWordWrap(width)
			case "most":
				glamWidth = glamour.WithWordWrap(int(float64(width) * 0.75))
			case "recomended":
				glamWidth = glamour.WithWordWrap(80)
			}

		case int64:
			w := int(lib.UserConfig.Reader.Size.(int64))
			glamWidth = glamour.WithWordWrap(w)
		default:
			log.Fatalf("invalid reader size: %v", lib.UserConfig.Reader.Size)
		}
		m.glam, _ = glamour.NewTermRenderer(
			glamour.WithEnvironmentConfig(),
			glamWidth,
		)

		if lib.UserConfig.Home == "mixed" {
			m.loadMixed()
		} else {
			m.loadHome()
		}

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

	if m.context.curr != "reader" && m.context.curr != "search" {
		view := lipgloss.JoinVertical(
			lipgloss.Top,
			m.table.View(),
			m.help.View(m.keys, m),
		)
		m.viewport.SetContent(view)
	} else if m.context.curr == "search" {
		m.filter, cmd = m.filter.Update(msg)
		cmds = append(cmds, cmd)

		view := lipgloss.JoinVertical(
			lipgloss.Top,
			m.filter.View(),
			m.table.View(),
			m.help.View(m.keys, m),
		)

		m.viewport.SetContent(view)
	}

	if m.context.curr == "reader" && m.viewport.ScrollPercent() >= lib.UserConfig.Reader.ReadThreshold {
		lib.MarkRead(m.context.feeds, m.context.feed.ID, m.context.post.ID)
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
