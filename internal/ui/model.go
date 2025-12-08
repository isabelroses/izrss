package ui

import (
	"log"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/rss"
	"github.com/isabelroses/izrss/internal/storage"
)

// Model is the main model for the application
type Model struct {
	help     HelpModel
	keys     keyMap
	glam     *glamour.TermRenderer
	context  context
	viewport viewport.Model
	filter   textinput.Model
	table    table.Model
	ready    bool

	// Dependencies
	cfg     *config.Config
	db      *storage.DB
	fetcher *rss.Fetcher
	styles  *Styles
}

// Init sets the initial state of the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("izrss"),
	)
}

// NewModel creates a new model with sensible defaults
func NewModel(cfg *config.Config, db *storage.DB, fetcher *rss.Fetcher) *Model {
	styles := NewStyles(cfg)

	t := table.New(table.WithFocused(true))
	t.SetStyles(TableStyles(cfg))

	f := textinput.New()
	f.Prompt = "Filter: "
	f.PromptStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229"))

	return &Model{
		context:  context{},
		viewport: viewport.Model{},
		table:    t,
		ready:    false,
		help:     NewHelp(styles),
		keys:     defaultKeyMap,
		filter:   f,
		cfg:      cfg,
		db:       db,
		fetcher:  fetcher,
		styles:   styles,
	}
}

// Update handles messages and updates the model
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
	framew, frameh := m.styles.Main.GetFrameSize()

	height := msg.Height - frameh
	width := msg.Width - framew

	m.table.SetWidth(width)
	m.table.SetHeight(height - lipgloss.Height(m.help.View(m.keys, m)))

	if !m.ready {
		m.viewport = viewport.New(width, height)

		m.setupGlamour(width)

		if m.cfg.Home == "mixed" {
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

func (m *Model) setupGlamour(width int) {
	var glamWidth glamour.TermRendererOption
	switch size := m.cfg.Reader.Size.(type) {
	case string:
		switch size {
		case "full", "fullscreen":
			glamWidth = glamour.WithWordWrap(width)
		case "most":
			glamWidth = glamour.WithWordWrap(int(float64(width) * 0.75))
		case "recomended":
			glamWidth = glamour.WithWordWrap(80)
		default:
			glamWidth = glamour.WithWordWrap(80)
		}
	case int64:
		glamWidth = glamour.WithWordWrap(int(size))
	default:
		log.Printf("invalid reader size: %v, using default", m.cfg.Reader.Size)
		glamWidth = glamour.WithWordWrap(80)
	}

	var glamTheme glamour.TermRendererOption
	switch m.cfg.Reader.Theme {
	case "environment":
		glamTheme = glamour.WithEnvironmentConfig()
	case "":
		glamTheme = glamour.WithAutoStyle()
	default:
		glamTheme = glamour.WithStylePath(m.cfg.Reader.Theme)
	}

	m.glam, _ = glamour.NewTermRenderer(
		glamTheme,
		glamWidth,
		glamour.WithChromaFormatter("terminal256"),
	)
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
	if m.context.curr == "reader" && m.context.prev == "mixed" && m.viewport.ScrollPercent() >= m.cfg.Reader.ReadThreshold {
		rss.MarkRead(m.context.feeds, m.context.feed.ID, m.context.post.ID)
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
