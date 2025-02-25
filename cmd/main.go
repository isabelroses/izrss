package cmd

import (
	"log"
	"sync"

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
	m.table.SetHeight(height - lipgloss.Height(m.help.View(m.keys, m)))

	if !m.ready {
		m.viewport = viewport.New(width, height)

		// we make this part mutli-threaded otherwise its really slow
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
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

			var glamTheme glamour.TermRendererOption
			if lib.UserConfig.Reader.Theme == "environment" {
				glamTheme = glamour.WithEnvironmentConfig()
			} else if lib.UserConfig.Reader.Theme != "" {
				glamTheme = glamour.WithStylePath(lib.UserConfig.Reader.Theme)
			} else {
				glamTheme = glamour.WithAutoStyle()
			}

			m.glam, _ = glamour.NewTermRenderer(
				glamTheme,
				glamWidth,
			)
		}()

		if lib.UserConfig.Home == "mixed" {
			m.loadMixed()
		} else {
			m.loadHome()
		}

		wg.Wait()
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

	// HACK: if the previous was mixed we never marked the post as read
	if m.context.curr == "reader" && m.context.prev == "mixed" && m.viewport.ScrollPercent() >= lib.UserConfig.Reader.ReadThreshold {
		lib.MarkRead(m.context.feeds, m.context.feed.ID, m.context.post.ID)
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
