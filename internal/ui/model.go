package ui

import (
	"log"
	"os"
	"runtime/debug"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/images"
	"github.com/isabelroses/izrss/internal/reader"
	"github.com/isabelroses/izrss/internal/rss"
	"github.com/isabelroses/izrss/internal/storage"
)

type screen int

const (
	screenMain screen = iota
	screenReader
)

type pane int

const (
	paneFeeds pane = iota
	panePosts
)

// Model is the application state.
type Model struct {
	cfg     *config.Config
	db      *storage.DB
	fetcher *rss.Fetcher
	styles  *Styles
	help    help.Model

	width, height int
	ready         bool
	started       bool // initial feed selection has been made

	screen   screen
	overlay  overlay
	palette  palette
	focus    pane
	postsFor string // the feed key the posts pane was built for
	feeds    rss.Feeds
	feedList list
	postList list
	refresh  refreshState
	toasts   toasts
	now      func() time.Time
	openURL  func(string) error

	// reader
	viewport viewport.Model
	open     openRef         // the post in the reader
	post     rss.Post        // a copy of it, for rendering
	doc      reader.Document // the open post, as last rendered
	docWidth []int           // the cell width of each of doc.Lines

	imgOn      bool
	imgFetcher *images.Fetcher
	ids        *images.IDs
	cell       images.Cell
	imgs       map[string]*postImage // images of imgPost, by resolved src
	imgPost    string
	imgToasted bool // the open post's image failures were reported

	warnings []string // config warnings, toasted at startup
}

// openRef identifies the post in the reader independently of row order.
type openRef struct{ feedURL, key string }

// NewModel creates a model; nothing touches the terminal or network here.
func NewModel(cfg *config.Config, db *storage.DB, fetcher *rss.Fetcher, imgFetcher *images.Fetcher, warnings []string) *Model {
	styles := NewStyles(cfg)
	m := &Model{
		cfg:      cfg,
		help:     newHelp(styles),
		db:       db,
		fetcher:  fetcher,
		styles:   styles,
		palette:  newPalette(),
		viewport: viewport.New(),
		now:      time.Now,
		openURL:  openURL,

		imgOn:      imgFetcher != nil && images.Enabled(cfg.Reader.Images, os.Getenv),
		imgFetcher: imgFetcher,
		ids:        images.NewIDs(),
		imgs:       map[string]*postImage{},
		warnings:   warnings,
	}
	// Assume a dark background until the terminal answers asynchronously; a
	// synchronous query blocks startup for up to 4s on some terminals.
	m.setBackground(true)
	m.rebuildFeeds()
	return m
}

// Init loads cached feeds for an instant first paint, then refreshes them over
// the network. The terminal background arrives as a tea.BackgroundColorMsg.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		minuteTick(),
		m.warningsCmd(),
		tea.Sequence(m.loadCachedFeeds(), m.refreshFeeds(m.allIndexes())),
	)
}

// warningsCmd turns config warnings into toasts.
func (m Model) warningsCmd() tea.Cmd {
	cmds := make([]tea.Cmd, len(m.warnings))
	for i, w := range m.warnings {
		cmds[i] = func() tea.Msg { return notifyWarn(w) }
	}
	return tea.Batch(cmds...)
}

// setBackground applies the styles that depend on the terminal background.
func (m *Model) setBackground(isDark bool) {
	st := textinput.DefaultStyles(isDark)
	st.Focused.Prompt = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.cfg.Colors.Accent))
	st.Cursor.Blink = false // a steady cursor needs no redraw timer
	m.palette.input.SetStyles(st)
}

// Update routes messages to the current screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	defer logPanic()
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		m.refreshCellSize()
		if m.screen == screenReader {
			cmds = append(cmds, m.resizeImages())
			m.rerenderReader()
		}
	case tea.BackgroundColorMsg:
		m.setBackground(msg.IsDark())
	case tea.ResumeMsg:
		if m.screen == screenReader {
			cmds = append(cmds, m.reuploadImages())
		}
	case tea.KeyPressMsg:
		switch {
		case m.overlay == overlayHelp:
			m.overlay = overlayNone // any key closes help
		case m.overlay == overlayPalette:
			cmds = append(cmds, m.paletteKey(msg))
		case key.Matches(msg, bindings.Search): // both screens
			cmds = append(cmds, m.openPalette())
		case key.Matches(msg, bindings.Help):
			m.overlay = overlayHelp
		case m.screen == screenReader:
			cmds = append(cmds, m.readerKey(msg))
		default:
			cmds = append(cmds, m.mainKey(msg))
		}
	case tea.MouseWheelMsg:
		if m.screen == screenReader {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd, m.markReadIfScrolled())
		}
	case feedsRefreshedMsg:
		for _, e := range msg.errs {
			cmds = append(cmds, m.notify(toastError, e))
		}
		m.feeds = msg.feeds
		m.feedsChanged()
		if !msg.at.IsZero() {
			m.refresh.at = msg.at
		}
		m.rebuildFeeds()
		m.selectInitialFeed()
	case refreshStartedMsg:
		m.refresh.total += msg.total
	case feedLoadedMsg:
		cmds = append(cmds, m.feedLoaded(msg))
	case minuteTickMsg:
		cmds = append(cmds, minuteTick())
	case imageLoadedMsg:
		cmds = append(cmds, m.imageLoaded(msg), m.imageFailureToast())
	case imageReadyMsg:
		m.imageReady(msg)
	case toastExpiredMsg:
		m.toasts.expire(msg)
	case notifyMsg:
		cmds = append(cmds, m.notify(msg.kind, msg.text))
	}
	return m, tea.Batch(cmds...)
}

// resize lays out both screens for a width×height terminal.
func (m *Model) resize(width, height int) {
	m.width, m.height = width, height
	m.viewport.SetWidth(width)
	m.viewport.Style = pageStyle(width)
	m.viewport.SetHeight(max(height-2, 1)) // header and hint lines; the page style pads inside
	body := max(height-1, 0)               // the hint/status line
	m.feedList.setHeight(body)
	m.postList.setHeight(body)
	m.ready = true
}

func (m *Model) selectInitialFeed() {
	if m.started || len(m.feeds) == 0 {
		return
	}
	m.started = true
	if m.cfg.Home != "mixed" && len(m.feedList.items) > 1 {
		m.feedList.move(1 - m.feedList.cursor)
		m.rebuildPosts()
	}
}

// quit saves read state and exits.
func (m *Model) quit() tea.Cmd {
	if err := m.feeds.WriteTracking(m.db); err != nil {
		log.Printf("error writing tracking: %v", err)
	}
	return tea.Quit
}

// logPanic records a panic and its stack in the log file, then lets it
// continue so Bubble Tea restores the terminal. Use as `defer logPanic()`.
func logPanic() {
	if r := recover(); r != nil {
		log.Printf("panic: %v\n%s", r, debug.Stack())
		panic(r)
	}
}
