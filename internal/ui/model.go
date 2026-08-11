package ui

import (
	"log"
	"os"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

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

	cfg       *config.Config
	db        *storage.DB
	fetcher   *rss.Fetcher
	styles    *Styles
	glamStyle string
}

// Init loads feeds from cache for an instant first paint, then refreshes them
// over the network, so the UI never blocks on startup.
func (m Model) Init() tea.Cmd {
	return tea.Sequence(m.loadCachedFeeds(), m.refreshAll())
}

// NewModel creates a new model with sensible defaults
func NewModel(cfg *config.Config, db *storage.DB, fetcher *rss.Fetcher) *Model {
	styles := NewStyles(cfg)

	t := table.New(table.WithFocused(true))
	t.SetStyles(TableStyles(cfg))

	// Resolve the background now, before bubbletea owns stdin: querying it later
	// means bubbletea swallows the terminal's reply and we block ~5s waiting on a
	// response that never arrives. Glamour v2 has no auto-style, so we pick one.
	isDark := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	glamStyle := "light"
	if isDark {
		glamStyle = "dark"
	}

	f := textinput.New()
	f.Prompt = "Filter: "
	fStyles := textinput.DefaultStyles(isDark)
	fStyles.Focused.Prompt = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229"))
	f.SetStyles(fStyles)

	return &Model{
		context:   context{},
		viewport:  viewport.New(),
		table:     t,
		ready:     false,
		help:      NewHelp(styles),
		keys:      defaultKeyMap,
		filter:    f,
		cfg:       cfg,
		db:        db,
		fetcher:   fetcher,
		styles:    styles,
		glamStyle: glamStyle,
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
	case tea.KeyPressMsg:
		m, cmd = m.handleKeys(msg)
		cmds = append(cmds, cmd)
	case feedsRefreshedMsg:
		m.context.feeds = msg.feeds
		m.reloadList()
	case feedRefreshedMsg:
		if msg.id >= 0 && msg.id < len(m.context.feeds) {
			m.context.feeds[msg.id].Posts = msg.posts
			if err := m.context.feeds.ReadTracking(m.db); err != nil {
				log.Printf("error reading tracking: %v", err)
			}
		}
		m.reloadList()
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
		m.viewport = viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))

		m.setupGlamour(width)

		if m.cfg.Home == "mixed" {
			m.loadMixed()
		} else {
			m.loadHome()
		}

		m.ready = true
	} else {
		m.viewport.SetWidth(width)
		m.viewport.SetHeight(height)
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
		glamTheme = glamour.WithStandardStyle(m.glamStyle)
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

	if m.context.curr == "reader" && m.viewport.ScrollPercent() >= m.cfg.Reader.ReadThreshold {
		post := &m.context.feeds[m.context.feed.ID].Posts[m.context.post.ID]
		if !post.Read {
			rss.MarkRead(m.context.feeds, m.context.feed.ID, m.context.post.ID)
			if err := m.context.feeds.WritePostTracking(m.db, m.context.feed.ID, m.context.post.ID); err != nil {
				log.Printf("error writing tracking: %v", err)
			}
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
