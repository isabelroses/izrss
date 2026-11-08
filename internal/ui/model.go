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
	"github.com/isabelroses/izrss/internal/imgsession"
	"github.com/isabelroses/izrss/internal/reader"
	"github.com/isabelroses/izrss/internal/readstate"
	"github.com/isabelroses/izrss/internal/refresh"
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

	screen     screen
	overlay    overlay
	palette    palette
	focus      pane
	postsFor   string // the feed key the posts pane was built for
	feeds      *rss.FeedSet
	read       *readstate.State
	feedList   list
	postList   list
	unread     int // unread posts in all feeds, as of rebuildFeeds
	feedTitleW int // the widest feeds-pane title, as of rebuildFeeds
	refresh    *refresh.Refresher
	toasts     toasts
	now        func() time.Time
	openURL    func(string) error

	// reader
	viewport viewport.Model
	open     rss.PostRef     // the post in the reader
	post     rss.Post        // a copy of it, for rendering
	doc      reader.Document // the open post, as last rendered
	docWidth []int           // the cell width of each of doc.Lines

	images *imgsession.Session // the open post's images
	cell   images.Cell         // the terminal's cell size, when images are on

	warnings []string // config warnings, toasted at startup
}

// NewModel creates a model; nothing touches the terminal or network here.
func NewModel(cfg *config.Config, db *storage.DB, fetcher *rss.Fetcher, imgFetcher *images.Fetcher, warnings []string) *Model {
	styles := NewStyles(cfg)
	feeds := rss.NewFeedSet()
	var loader imgsession.Loader // a nil *Fetcher must not become a non-nil interface
	if imgFetcher != nil {
		loader = imgFetcher
	}
	m := &Model{
		cfg:      cfg,
		help:     newHelp(styles),
		db:       db,
		fetcher:  fetcher,
		styles:   styles,
		palette:  newPalette(),
		feeds:    feeds,
		refresh:  refresh.New(feeds),
		read:     readstate.New(db),
		viewport: viewport.New(),
		now:      time.Now,
		openURL:  openURL,

		images:   imgsession.New(loader, loader != nil && images.Enabled(cfg.Reader.Images, os.Getenv)),
		warnings: warnings,
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
		tea.Sequence(m.loadCachedFeeds(), func() tea.Msg { return refreshMsg{urls: m.cfg.Urls} }),
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
		first := !m.ready
		m.resize(msg.Width, msg.Height)
		m.refreshCellSize()
		if first { // the first frame follows; uploads may start
			for _, o := range m.images.FrameDrawn() {
				cmds = append(cmds, m.imageOutcome(o))
			}
		}
		if m.screen == screenReader {
			cmds = append(cmds, rawCmd(m.images.Resized(m.imageLayout())))
			m.rerenderReader()
		}
	case tea.BackgroundColorMsg:
		m.setBackground(msg.IsDark())
	case tea.ResumeMsg:
		if m.screen == screenReader {
			cmds = append(cmds, rawCmd(m.images.Resume()))
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
		m.feeds.Reset(msg.feeds)
		if msg.read != nil {
			m.read.Restore(msg.read)
		}
		m.researchPalette()
		m.refresh.SetAt(msg.at)
		m.rebuildFeeds()
		m.selectInitialFeed()
	case refreshMsg:
		cmds = append(cmds, m.refreshFeeds(msg.urls))
	case feedLoadedMsg:
		cmds = append(cmds, m.feedLoaded(msg))
	case minuteTickMsg:
		cmds = append(cmds, minuteTick())
	case imageLoadedMsg:
		cmds = append(cmds, m.imageOutcome(m.images.Loaded(msg.Result)))
	case imageReadyMsg:
		if m.images.Uploaded(msg.post, msg.src) {
			m.rerenderReader()
		}
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
	m.viewport.SetHeight(max(height-2-pageGap(height), 1)) // header and hint lines, the gap above the hint; the page style pads inside
	body := max(height-1, 0)                               // the hint/status line
	m.feedList.setHeight(body)
	m.postList.setHeight(body)
	m.ready = true
}

func (m *Model) selectInitialFeed() {
	if m.started || m.feeds.Len() == 0 {
		return
	}
	m.started = true
	if m.cfg.Home != "mixed" && len(m.feedList.items) > 1 {
		m.feedList.move(1 - m.feedList.cursor)
		m.rebuildPosts()
	}
}

// logPanic records a panic and its stack in the log file, then lets it
// continue so Bubble Tea restores the terminal. Use as `defer logPanic()`.
func logPanic() {
	if r := recover(); r != nil {
		log.Printf("panic: %v\n%s", r, debug.Stack())
		panic(r)
	}
}
