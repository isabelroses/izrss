package ui

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/images"
	"github.com/isabelroses/izrss/internal/imgsession"
	"github.com/isabelroses/izrss/internal/reader"
	"github.com/isabelroses/izrss/internal/rss"
)

// readerPage is the open post, full-screen: its rendered document, the
// scrolling viewport over it, its images and the page layout. Model tells it
// what happened (opened, resized, scrolled, an image arrived, the post was
// refreshed) and draws it; the page keeps its own document, scroll position
// and images consistent with each other.
type readerPage struct {
	cfg        *config.Reader
	dateFormat string
	theme      reader.Theme
	images     *imgsession.Session

	shown         bool
	width, height int
	framed        bool        // the first frame is out: image uploads may start
	cell          images.Cell // the terminal's cell size, when images are on

	viewport viewport.Model
	ref      rss.PostRef     // the open post
	post     rss.Post        // a copy of it, for rendering
	doc      reader.Document // the open post, as last rendered
	docWidth []int           // the cell width of each of doc.Lines
}

// imageLoadedMsg is a finished load, already encoded by its command.
type imageLoadedMsg struct{ imgsession.Result }

// imageReadyMsg says an image's upload was written.
type imageReadyMsg struct{ post, src string }

const (
	readerMargin = 2 // blank columns each side of the reader page
	// docMargin indents the post inside the page, as glamour's document
	// margin does; the header and hint bar stay at the page margin.
	docMargin = 2
)

func newReaderPage(cfg *config.Reader, dateFormat string, theme reader.Theme, imgs *imgsession.Session) readerPage {
	return readerPage{cfg: cfg, dateFormat: dateFormat, theme: theme, images: imgs, viewport: viewport.New()}
}

// Shown reports whether the page is open.
func (p *readerPage) Shown() bool { return p.shown }

// Ref is the open post.
func (p *readerPage) Ref() rss.PostRef { return p.ref }

// Link is the open post's link.
func (p *readerPage) Link() string { return p.post.Link }

// Open shows post at the top and starts loading its images.
func (p *readerPage) Open(ref rss.PostRef, post rss.Post) tea.Cmd {
	p.shown, p.ref, p.post = true, ref, post
	p.viewport.Style = p.pageStyle(p.width) // the reader width may have been changed
	// The terminal may have been resized since this post's images were fitted.
	place := rawCmd(p.images.Open(sessionKey(ref), p.imageLayout()))
	p.viewport.SetYOffset(0)
	p.render()
	return tea.Batch(place, imageJobs(p.images.Fetch(p.doc.Images, p.imageLayout())))
}

// Close hides the page; its images stay uploaded for reopening.
func (p *readerPage) Close() { p.shown = false }

// Refresh swaps in a new copy of the open post, as after a feed refresh.
func (p *readerPage) Refresh(post rss.Post) {
	p.post = post
	p.rerender()
}

// Resize lays the page out for a width×height terminal.
func (p *readerPage) Resize(width, height int) tea.Cmd {
	p.width, p.height = width, height
	p.viewport.SetWidth(width)
	p.viewport.Style = p.pageStyle(width)
	p.viewport.SetHeight(max(height-2-pageGap(height), 1)) // header and hint lines, the gap above the hint; the page style pads inside
	if p.images.Enabled() {
		p.cell = images.CellSize(int(os.Stdout.Fd()))
	}
	var cmds []tea.Cmd
	if !p.framed { // the first frame follows; uploads may start
		p.framed = true
		for _, o := range p.images.FrameDrawn() {
			cmds = append(cmds, p.outcome(o))
		}
	}
	if p.shown {
		cmds = append(cmds, rawCmd(p.images.Resized(p.imageLayout())))
		p.rerender()
	}
	return tea.Batch(cmds...)
}

// Scroll passes a key or wheel event to the viewport.
func (p *readerPage) Scroll(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

// ReadEnough reports whether the open post has been scrolled past the read
// threshold; short posts qualify as soon as they open.
func (p *readerPage) ReadEnough() bool {
	return p.shown && p.viewport.ScrollPercent() >= p.cfg.ReadThreshold
}

// ImageLoaded takes a finished image load. failed is the number of the
// post's images that couldn't be loaded, reported once when the last one
// finishes.
func (p *readerPage) ImageLoaded(r imgsession.Result) (cmd tea.Cmd, failed int) {
	o := p.images.Loaded(r)
	return p.outcome(o), o.Failed
}

// ImageReady says an image's upload was written, so it can be drawn.
func (p *readerPage) ImageReady(msg imageReadyMsg) {
	if p.images.Uploaded(msg.post, msg.src) {
		p.rerender()
	}
}

// Resume re-sends the images after a suspend dropped the terminal's store.
func (p *readerPage) Resume() tea.Cmd {
	if !p.shown {
		return nil
	}
	return rawCmd(p.images.Resume())
}

// outcome writes what the image session asks (confirming the upload after)
// and renders again if it must.
func (p *readerPage) outcome(o imgsession.Outcome) tea.Cmd {
	if o.Rerender {
		p.rerender()
	}
	if o.Write == "" {
		return nil
	}
	write := tea.Raw(o.Write)
	if !o.Confirm {
		return write
	}
	post, src := o.Post, o.Src
	return tea.Sequence(write, func() tea.Msg { return imageReadyMsg{post: post, src: src} })
}

// render renders the open post into the viewport.
func (p *readerPage) render() {
	p.doc = reader.Render(p.post.Content, p.opts())
	p.viewport.SetContent(p.doc.String())
	p.docWidth = make([]int, len(p.doc.Lines))
	for i, line := range p.doc.Lines {
		p.docWidth[i] = ansi.StringWidth(line)
	}
}

// rerender re-renders the open post, keeping the line being read in place
// when images above it change height.
func (p *readerPage) rerender() {
	if !p.shown {
		return
	}
	old := p.doc
	offset := p.viewport.YOffset()
	p.render()
	for i, img := range p.doc.Images {
		if i < len(old.Images) && old.Images[i].Src == img.Src && old.Images[i].Line < offset {
			offset += img.Height - old.Images[i].Height
		}
	}
	p.viewport.SetYOffset(offset)
}

func (p *readerPage) opts() reader.Options {
	ready, failed := p.images.Display()
	return reader.Options{
		Width:      p.textWidth(),
		Margin:     docMargin,
		Theme:      p.theme,
		BaseURL:    p.post.Link,
		ShowImages: p.images.Enabled(),
		Ready:      ready,
		Failed:     failed,
	}
}

// margin is the blank columns left of the text column; the header aligns
// with it.
func (p *readerPage) margin() int { return p.viewport.Style.GetPaddingLeft() }

// textWidth is the column the open post wraps at, inside the page margins.
func (p *readerPage) textWidth() int {
	return p.cfg.Width.Of(p.viewport.Width() - p.viewport.Style.GetHorizontalFrameSize())
}

// contentWidth is the column images are fitted to: the text width inside the
// document margin.
func (p *readerPage) contentWidth() int {
	return reader.ContentWidth(reader.Options{Width: p.textWidth(), Margin: docMargin})
}

// imageMaxRows is the configured image height, or the reader's height.
func (p *readerPage) imageMaxRows() int {
	if h := p.cfg.ImageMaxHeight; h > 0 {
		return h
	}
	return max(p.viewport.Height()-p.viewport.Style.GetVerticalFrameSize(), 1)
}

// imageLayout is the room the open post's images are fitted to.
func (p *readerPage) imageLayout() imgsession.Layout {
	return imgsession.Layout{Cols: p.contentWidth(), Rows: p.imageMaxRows(), Cell: p.cell}
}

// View draws the screen: the header, the page, the gap above the hint line,
// then status, the hint line itself. feed names the open post's feed. The
// frame is joined once; it is large, so a second copy shows per frame.
func (p *readerPage) View(feed string, st *Styles, status string) string {
	lines := []string{p.header(feed, st), p.body()}
	if pageGap(p.height) > 0 {
		lines = append(lines, strings.Repeat(" ", p.width))
	}
	return strings.Join(append(lines, status), "\n")
}

// header is the post title, then feed and date, with the scroll position on
// the right.
func (p *readerPage) header(feed string, st *Styles) string {
	var meta []string
	if feed != "" {
		meta = append(meta, feed)
	}
	if date := p.post.Date(p.dateFormat); date != "" {
		meta = append(meta, date)
	}
	margin := p.margin()
	pct := fmt.Sprintf("%3.f%%", p.viewport.ScrollPercent()*100)
	room := max(p.width-2*margin-len(pct)-2, 0)

	title := ansi.Truncate(p.post.Title, room, "…")
	rest := ""
	if len(meta) > 0 {
		rest = ansi.Truncate(" · "+strings.Join(meta, " · "), room-ansi.StringWidth(title), "…")
	}
	pad := strings.Repeat(" ", margin)
	gap := strings.Repeat(" ", max(p.width-2*margin-ansi.StringWidth(title)-ansi.StringWidth(rest)-len(pct), 1))
	line := pad + st.Header.Render(title) + st.Hint.Render(rest) + gap + st.Hint.Render(pct) + pad
	return ansi.Truncate(line, p.width, "")
}

// body draws the viewport as viewport.View does, but from line widths
// measured once per render: viewport.View re-measures them every frame, which
// is slow for image rows.
func (p *readerPage) body() string {
	vp := p.viewport
	left, right, top := vp.Style.GetPaddingLeft(), vp.Style.GetPaddingRight(), vp.Style.GetPaddingTop()
	width, height := vp.Width()-left-right, vp.Height()-top
	if width <= 0 || height <= 0 || vp.TotalLineCount() != len(p.doc.Lines) || vp.Style.GetPaddingBottom() != 0 {
		return vp.View()
	}
	first := vp.YOffset()
	visible := p.docWidth[min(first, len(p.docWidth)):min(first+height, len(p.docWidth))]
	if slices.ContainsFunc(visible, func(w int) bool { return w > width }) {
		return vp.View()
	}

	blank := strings.Repeat(" ", vp.Width())
	margin := strings.Repeat(" ", left)
	lines := make([]string, 0, top+height)
	for range top {
		lines = append(lines, blank)
	}
	for i := range height {
		if i >= len(visible) {
			lines = append(lines, blank)
			continue
		}
		lines = append(lines, margin+p.doc.Lines[first+i]+strings.Repeat(" ", width-visible[i]+right))
	}
	return strings.Join(lines, "\n")
}

// pageStyle pads the reader viewport: a blank row under the header, a
// margin each side, and the text column centred in the room left over when
// the configured width is narrower.
func (p *readerPage) pageStyle(width int) lipgloss.Style {
	side := pageSide(width)
	room := max(width-2*side, 0)
	extra := room - p.cfg.Width.Of(room)
	return lipgloss.NewStyle().Padding(1, side+extra-extra/2, 0, side+extra/2)
}

// pageSide is the reader page's margin each side, which narrow terminals
// don't get; the hint line keeps to it.
func pageSide(width int) int {
	if width < 40 {
		return 0
	}
	return readerMargin
}

// pageGap is the blank rows between the article and the hint line: one, unless
// the terminal is too short to spare it.
func pageGap(height int) int {
	if height < 8 {
		return 0
	}
	return 1
}

// sessionKey names the open post to the image session.
func sessionKey(ref rss.PostRef) string { return ref.FeedURL + "\x00" + ref.Key }

// imageJobs turns the session's loads into commands.
func imageJobs(jobs []imgsession.Job) tea.Cmd {
	cmds := make([]tea.Cmd, len(jobs))
	for i, job := range jobs {
		cmds[i] = func() tea.Msg { return imageLoadedMsg{job()} }
	}
	return tea.Batch(cmds...)
}

// rawCmd writes s straight to the terminal, or does nothing when s is empty.
func rawCmd(s string) tea.Cmd {
	if s == "" {
		return nil
	}
	return tea.Raw(s)
}
