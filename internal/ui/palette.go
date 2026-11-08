package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/library"
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

// palette is the pop-up search box: live results across every feed. It
// searches the library itself and keeps its results current as the library
// changes (Sync); Model shows it, passes it keys, and opens what it picks.
type palette struct {
	lib        *library.Library
	input      textinput.Model
	hits       []rss.PostRef
	cursor     int
	searchedAt int // the library version hits were searched in
}

func newPalette(lib *library.Library) palette {
	in := textinput.New()
	in.Prompt = "/ "
	in.Placeholder = "search every post"
	return palette{lib: lib, input: in}
}

// Open empties the palette for a new search.
func (p *palette) Open() tea.Cmd {
	p.input.Reset()
	p.hits, p.cursor, p.searchedAt = nil, 0, p.lib.Version()
	return p.input.Focus()
}

// Key handles a key while the palette is open. done reports that it closed,
// and pick is the post chosen then, if one was.
func (p *palette) Key(msg tea.KeyPressMsg) (cmd tea.Cmd, pick rss.PostRef, done bool) {
	switch msg.String() {
	case "esc":
		done = true
	case "enter":
		if p.cursor < len(p.hits) {
			if _, ok := p.lib.Entry(p.hits[p.cursor]); ok { // else gone in a refresh not yet searched
				pick, done = p.hits[p.cursor], true
			}
		}
	case "up", "ctrl+p":
		p.cursor = max(p.cursor-1, 0)
	case "down", "ctrl+n":
		p.cursor = min(p.cursor+1, max(len(p.hits)-1, 0))
	default:
		before := p.input.Value()
		p.input, cmd = p.input.Update(msg)
		if p.input.Value() != before {
			p.search()
			p.cursor = 0
		}
	}
	if done {
		p.input.Blur()
	}
	return cmd, pick, done
}

// Sync re-runs the query if the library changed since it last ran. While a
// refresh is under way (settled is false) it waits: each feed arriving would
// otherwise search every post again.
func (p *palette) Sync(settled bool) {
	if !settled || p.lib.Version() == p.searchedAt {
		return
	}
	p.search()
	p.cursor = min(p.cursor, max(len(p.hits)-1, 0))
}

func (p *palette) search() {
	p.hits, p.searchedAt = p.lib.Search(p.input.Value()), p.lib.Version()
}

func paletteWidth(width int) int { return min(paletteMaxWidth, width*8/10) }

// View draws the palette box, paletteWidth(width) cells wide.
func (p *palette) View(width int, st *Styles) string {
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
		e, _ := p.lib.Entry(ref)
		items[i] = item{title: e.Post.Title, right: e.Feed, strong: !e.Read}
	}
	rows := list{items: items, cursor: p.cursor - first, height: min(paletteRows, len(p.hits))}
	lines = append(lines, rows.view(inner, true, st.List)...)

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
	return box(strings.Join(lines, "\n"), boxW, st.List.accent)
}

// box frames content in a rounded border exactly width cells wide.
func box(content string, width int, border color.Color) string {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(width).BorderForeground(border).Render(content)
}

// helpView is the help overlay: every key of the current screen, in columns.
func (m Model) helpView() string {
	title, foot := "Keys", "h j k l work too · any key closes"
	if m.page.Shown() {
		title, foot = "Reader keys", "any key closes"
	}
	body := m.help.FullHelpView(bindings.fullHelp(m.page.Shown()))
	content := strings.Join([]string{m.styles.Header.Render(title), "", body, "", m.styles.Hint.Render(foot)}, "\n")
	width := min(lipgloss.Width(content)+4, m.width) // border and padding
	return box(content, width, hex(m.cfg.Colors.Accent))
}
