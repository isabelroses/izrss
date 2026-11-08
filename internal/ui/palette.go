package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/readstate"
	"github.com/isabelroses/izrss/internal/rss"
)

type overlay int

const (
	overlayNone overlay = iota
	overlayHelp
	overlayPalette
)

const (
	paletteRows     = 10
	paletteMaxWidth = 80
)

// palette is the pop-up search box: live results across every feed.
type palette struct {
	input  textinput.Model
	hits   []rss.PostRef
	cursor int
	index  *searchIndex // built on the first search; nil when stale
}

func newPalette() palette {
	in := textinput.New()
	in.Prompt = "/ "
	in.Placeholder = "search every post"
	return palette{input: in}
}

func (m *Model) openPalette() tea.Cmd {
	m.overlay = overlayPalette
	m.palette.input.Reset()
	m.palette.hits, m.palette.cursor, m.palette.index = nil, 0, nil
	return m.palette.input.Focus()
}

func (m *Model) paletteKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.closePalette()
		return nil
	case "enter":
		if m.palette.cursor >= len(m.palette.hits) {
			return nil
		}
		ref := m.palette.hits[m.palette.cursor]
		if _, ok := m.feeds.Post(ref); !ok { // the feed was refreshed since
			return nil
		}
		m.closePalette()
		return m.openReader(ref)
	case "up", "ctrl+p":
		m.palette.cursor = max(m.palette.cursor-1, 0)
		return nil
	case "down", "ctrl+n":
		m.palette.cursor = min(m.palette.cursor+1, max(len(m.palette.hits)-1, 0))
		return nil
	}
	before := m.palette.input.Value()
	var cmd tea.Cmd
	m.palette.input, cmd = m.palette.input.Update(msg)
	if q := m.palette.input.Value(); q != before {
		if m.palette.index == nil {
			m.palette.index = newSearchIndex(m.feeds.All())
		}
		m.palette.hits = m.palette.index.search(q)
		m.palette.cursor = 0
	}
	return cmd
}

// feedsChanged keeps the palette's results in step with m.feeds: an open
// palette re-runs its query on a fresh index, a closed one drops the index.
func (m *Model) feedsChanged() {
	m.palette.index = nil
	m.researchPalette()
}

// feedChanged is feedsChanged when only the feed with url changed (or was
// added): an open palette's index re-indexes just that feed.
func (m *Model) feedChanged(url string) {
	if m.overlay != overlayPalette || m.palette.index == nil {
		m.feedsChanged()
		return
	}
	m.palette.index.update(m.feeds.All(), url)
	m.researchPalette()
}

// researchPalette re-runs an open palette's query over m.feeds.
func (m *Model) researchPalette() {
	if m.overlay != overlayPalette {
		m.palette.index = nil
		return
	}
	if q := m.palette.input.Value(); q != "" {
		if m.palette.index == nil {
			m.palette.index = newSearchIndex(m.feeds.All())
		}
		m.palette.hits = m.palette.index.search(q)
	} else {
		m.palette.hits = nil
	}
	m.palette.cursor = min(m.palette.cursor, max(len(m.palette.hits)-1, 0))
}

// closePalette hides the palette and frees its search index.
func (m *Model) closePalette() {
	m.overlay = overlayNone
	m.palette.input.Blur()
	m.palette.index = nil
}

func paletteWidth(width int) int { return min(paletteMaxWidth, width*8/10) }

// view renders the palette box, paletteWidth(width) cells wide.
func (p palette) view(feeds *rss.FeedSet, read *readstate.State, width int, th listTheme, st *Styles) string {
	boxW := paletteWidth(width)
	inner := max(boxW-4, 1) // rounded border and one cell of padding each side
	// Text lines are indented one cell to line up with the result rows,
	// which pad themselves.
	in := p.input
	in.SetWidth(max(inner-ansi.StringWidth(in.Prompt)-3, 1))
	lines := []string{" " + ansi.Truncate(in.View(), inner-2, ""), " " + st.Border.Render(strings.Repeat("─", max(inner-2, 0)))}

	// Show a window of rows that keeps the cursor visible.
	first := max(min(p.cursor-paletteRows+1, len(p.hits)-paletteRows), 0)
	first = min(first, p.cursor)
	// Only the visible rows are built; a broad query can hit every post.
	visible := p.hits[first:min(first+paletteRows, len(p.hits))]
	items := make([]item, len(visible))
	for i, ref := range visible {
		feed, _ := feeds.Feed(ref.FeedURL)
		post, _ := feeds.Post(ref)
		items[i] = item{title: post.Title, right: feed.Name(), strong: !read.IsRead(ref.FeedURL, ref.Key)}
	}
	rows := list{items: items, cursor: p.cursor - first, height: min(paletteRows, len(p.hits))}
	lines = append(lines, rows.view(inner, true, th)...)

	var foot string
	switch {
	case p.input.Value() == "":
		foot = "type to search titles and content"
	case len(p.hits) == 0:
		foot = "No results"
	case len(p.hits) == 1:
		foot = "1 result"
	default:
		foot = fmt.Sprintf("%d results", len(p.hits))
	}
	lines = append(lines, " "+st.Hint.Render(ansi.Truncate(foot, max(inner-2, 0), "…")))
	return box(strings.Join(lines, "\n"), boxW, th.accent)
}

// box frames content in a rounded border exactly width cells wide.
func box(content string, width int, border color.Color) string {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(width).BorderForeground(border).Render(content)
}

// helpView is the help overlay: every key of the current screen, in columns.
func (m Model) helpView() string {
	title, foot := "Keys", "h j k l work too · any key closes"
	if m.screen == screenReader {
		title, foot = "Reader keys", "any key closes"
	}
	body := m.help.FullHelpView(bindings.fullHelp(m.screen))
	content := strings.Join([]string{m.styles.Header.Render(title), "", body, "", m.styles.Hint.Render(foot)}, "\n")
	width := min(lipgloss.Width(content)+4, m.width) // border and padding
	return box(content, width, hex(m.cfg.Colors.Accent))
}
