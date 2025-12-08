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

// FeedLoadedMsg is sent when a feed has been loaded
type FeedLoadedMsg struct {
	Feed rss.Feed
}

// BatchFeedsLoadedMsg is sent when all feeds are loaded from cache at once
type BatchFeedsLoadedMsg struct {
	Feeds rss.Feeds
}

// AllFeedsLoadedMsg is sent when all feeds have been loaded
type AllFeedsLoadedMsg struct{}

// Model is the main model for the application
type Model struct {
	help        HelpModel
	keys        keyMap
	glam        *glamour.TermRenderer
	context     context
	viewport    viewport.Model
	filter      textinput.Model
	table       table.Model
	ready       bool
	loading     bool
	loadedCount int
	totalCount  int

	// Dependencies
	cfg     *config.Config
	db      *storage.DB
	fetcher *rss.Fetcher
	styles  *Styles
}

// Init sets the initial state of the model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.SetWindowTitle("izrss"),
		tea.WindowSize(), // Get actual terminal size immediately
	}

	// Start loading feeds asynchronously if in loading mode
	if m.loading && m.loadedCount == 0 {
		cmds = append(cmds, m.loadFeedsCmd())
	}

	return tea.Batch(cmds...)
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
		context:     context{curr: "home"},
		viewport:    viewport.Model{},
		table:       t,
		ready:       false,
		loading:     false,
		loadedCount: 0,
		totalCount:  len(cfg.Urls),
		help:        NewHelp(styles),
		keys:        defaultKeyMap,
		filter:      f,
		cfg:         cfg,
		db:          db,
		fetcher:     fetcher,
		styles:      styles,
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
	case FeedLoadedMsg:
		m.context.feeds = append(m.context.feeds, msg.Feed)
		m.loadedCount++
		m.loadHome()
	case BatchFeedsLoadedMsg:
		m.context.feeds = msg.Feeds
		m.loadedCount = len(msg.Feeds)
		m.loading = false
		m.readTracking()
		m.loadHome()
	case AllFeedsLoadedMsg:
		m.loading = false
		m.readTracking()
		m.context.feeds = m.context.feeds.Sort(m.cfg.Urls)
		m.loadHome()
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
		m.ready = true
	} else {
		m.viewport.Width = width
		m.viewport.Height = height
	}

	m.setupGlamour(width)

	// Refresh the current view with new dimensions
	m.loadHome()

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

	// Auto-mark post as read when scrolled past the threshold
	if m.context.curr == "reader" && m.viewport.ScrollPercent() >= m.cfg.Reader.ReadThreshold {
		post := &m.context.feeds[m.context.feed.ID].Posts[m.context.post.ID]
		if !post.Read {
			rss.MarkRead(m.context.feeds, m.context.feed.ID, m.context.post.ID)
			if err := m.context.feeds.WriteTracking(m.db); err != nil {
				log.Printf("error writing tracking: %v", err)
			}
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
