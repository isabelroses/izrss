package ui

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/images"
	"github.com/isabelroses/izrss/internal/imgsession"
	"github.com/isabelroses/izrss/internal/library"
	"github.com/isabelroses/izrss/internal/refresh"
	"github.com/isabelroses/izrss/internal/rss"
)

type pane int

const (
	paneFeeds pane = iota
	panePosts
)

// Model is the application state. It routes messages to the screen showing
// and the overlays over it; each owns its own state.
type Model struct {
	cfg    *config.Config
	styles *Styles
	help   help.Model

	width, height int
	ready         bool

	lib     *library.Library
	refresh *refresh.Refresher
	main    mainPage   // the feeds and their posts
	page    readerPage // the open post
	overlay overlay
	palette palette
	toasts  toasts
	now     func() time.Time
	openURL func(string) error
}

// NewModel creates a model; nothing touches the terminal or network here.
// lib loads the cached feeds at Init and src refreshes them; with no src
// nothing is fetched, and with no imgs images are off.
func NewModel(cfg *config.Config, lib *library.Library, src refresh.Source, imgs imgsession.Loader) *Model {
	styles := NewStyles(cfg)
	m := &Model{
		cfg:     cfg,
		help:    newHelp(styles),
		styles:  styles,
		lib:     lib,
		refresh: refresh.New(lib, src),
		main:    newMainPage(lib, cfg.Home, cfg.DateFormat),
		palette: newPalette(lib),
		now:     time.Now,
		openURL: openURL,

		page: newReaderPage(&cfg.Reader, cfg.DateFormat, styles.Reader,
			imgsession.New(imgs, imgs != nil && images.Enabled(cfg.Reader.Images, os.Getenv))),
	}
	// Assume a dark background until the terminal answers asynchronously; a
	// synchronous query blocks startup for up to 4s on some terminals.
	m.setBackground(true)
	return m
}

// Init loads cached feeds for an instant first paint, then refreshes them over
// the network. The terminal background arrives as a tea.BackgroundColorMsg.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		minuteTick(),
		tea.Sequence(m.loadCachedFeeds(), func() tea.Msg { return refreshMsg{urls: m.cfg.Urls} }),
	)
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
		m.width, m.height, m.ready = msg.Width, msg.Height, true
		m.main.Resize(msg.Width, msg.Height)
		cmds = append(cmds, m.page.Resize(msg.Width, msg.Height))
	case tea.BackgroundColorMsg:
		m.setBackground(msg.IsDark())
	case tea.ResumeMsg:
		cmds = append(cmds, m.page.Resume())
	case tea.KeyPressMsg:
		switch {
		case m.overlay == overlayHelp:
			m.overlay = overlayNone // any key closes help
		case m.overlay == overlayPalette:
			cmd, pick, done := m.palette.Key(msg)
			cmds = append(cmds, cmd)
			if done {
				m.overlay = overlayNone
			}
			if pick != (rss.PostRef{}) {
				cmds = append(cmds, m.openReader(pick))
			}
		case key.Matches(msg, bindings.Search): // both screens
			m.overlay = overlayPalette
			cmds = append(cmds, m.palette.Open())
		case key.Matches(msg, bindings.Help):
			m.overlay = overlayHelp
		case m.page.Shown():
			cmds = append(cmds, m.readerKey(msg))
		default:
			cmds = append(cmds, m.mainKey(msg))
		}
	case tea.MouseWheelMsg:
		if m.page.Shown() {
			cmds = append(cmds, m.page.Scroll(msg), m.markReadIfScrolled())
		}
	case cacheLoadedMsg:
		if err := msg.snap.MarksErr; err != nil {
			log.Printf("reading read status: %v", err)
			cmds = append(cmds, m.notify("couldn't load read status"))
		}
		m.lib.Restore(msg.snap)
		m.refresh.SetAt(msg.snap.Fetched())
	case refreshMsg:
		cmds = append(cmds, m.refreshFeeds(msg.urls))
	case feedLoadedMsg:
		cmds = append(cmds, m.feedLoaded(msg))
	case minuteTickMsg:
		cmds = append(cmds, minuteTick())
	case imageLoadedMsg:
		cmd, failed := m.page.ImageLoaded(msg.Result)
		cmds = append(cmds, cmd, m.imagesFailed(failed))
	case imageReadyMsg:
		m.page.ImageReady(msg)
	case toastExpiredMsg:
		m.toasts.expire(msg)
	}
	// Whatever changed the feeds or read state, the views follow here.
	m.main.Sync()
	if m.overlay == overlayPalette {
		_, running := m.refresh.Progress()
		m.palette.Sync(running == 0)
	}
	return m, tea.Batch(cmds...)
}

// imagesFailed toasts the open post's images that couldn't be loaded.
func (m *Model) imagesFailed(n int) tea.Cmd {
	switch {
	case n == 1:
		return m.notify("1 image in this post couldn't be loaded")
	case n > 1:
		return m.notify(fmt.Sprintf("%d images in this post couldn't be loaded", n))
	}
	return nil
}

// logPanic records a panic and its stack in the log file, then lets it
// continue so Bubble Tea restores the terminal. Use as `defer logPanic()`.
func logPanic() {
	if r := recover(); r != nil {
		log.Printf("panic: %v\n%s", r, debug.Stack())
		panic(r)
	}
}

// openReader shows the post ref addresses full-screen.
func (m *Model) openReader(ref rss.PostRef) tea.Cmd {
	e, ok := m.lib.Entry(ref)
	if !ok {
		return nil
	}
	return tea.Batch(m.page.Open(ref, e.Post), m.markReadIfScrolled())
}

func (m *Model) readerKey(msg tea.KeyPressMsg) tea.Cmd {
	k := bindings
	switch {
	case key.Matches(msg, k.Back):
		m.page.Close()
		return nil
	case key.Matches(msg, k.ForceQuit):
		return tea.Quit
	case key.Matches(msg, k.Toggle):
		ref := m.page.Ref()
		return m.setRead(ref, !m.lib.IsRead(ref))
	case key.Matches(msg, k.Browser):
		return m.openLink(m.page.Link())
	}
	return tea.Batch(m.page.Scroll(msg), m.markReadIfScrolled())
}

// markReadIfScrolled marks the open post read once it has been scrolled past
// the read threshold.
func (m *Model) markReadIfScrolled() tea.Cmd {
	if !m.page.ReadEnough() {
		return nil
	}
	return m.setRead(m.page.Ref(), true)
}

// mainKey handles a key on the main screen: the page moves itself, and the
// rest acts on what it has selected.
func (m *Model) mainKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.main.Key(msg) {
		return nil
	}
	k := bindings
	switch {
	case key.Matches(msg, k.Quit):
		return tea.Quit
	case key.Matches(msg, k.Open):
		if e, ok := m.main.SelectedPost(); ok {
			return m.openReader(e.Ref)
		}
	case key.Matches(msg, k.Toggle):
		if e, ok := m.main.SelectedPost(); ok {
			return m.setRead(e.Ref, !e.Read)
		}
	case key.Matches(msg, k.MarkFeed):
		return m.readStatusSaved(m.lib.MarkRead(m.main.SelectedFeeds()))
	case key.Matches(msg, k.Refresh):
		return m.refreshFeeds(m.main.SelectedFeeds())
	case key.Matches(msg, k.RefreshAll):
		return m.refreshFeeds(m.cfg.Urls)
	case key.Matches(msg, k.Browser):
		if e, ok := m.main.SelectedPost(); ok {
			return m.openLink(e.Post.Link)
		}
	}
	return nil
}

// setRead changes and saves one post's read state.
func (m *Model) setRead(ref rss.PostRef, read bool) tea.Cmd {
	return m.readStatusSaved(m.lib.SetRead(ref, read))
}

// readStatusSaved reports a failure to save read state.
func (m *Model) readStatusSaved(err error) tea.Cmd {
	if err == nil {
		return nil
	}
	log.Printf("saving read status: %v", err)
	return m.notify("couldn't save read status")
}

func (m *Model) openLink(link string) tea.Cmd {
	if err := m.openURL(link); err != nil {
		return m.notify("couldn't open link: " + err.Error())
	}
	return nil
}
