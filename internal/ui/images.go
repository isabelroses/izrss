package ui

import (
	"log"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/isabelroses/izrss/internal/images"
	"github.com/isabelroses/izrss/internal/reader"
)

// postImage is one image of the post currently in the reader.
type postImage struct {
	prep     images.Prepared // sizes only; pixels are dropped once encoded
	id       int
	upload   string // the kitty upload escape, kept for re-sending
	loaded   bool
	uploaded bool
	failed   bool

	rows             []string // placeholder rows, cached for rowCols×rowRows
	rowCols, rowRows int
}

// imageLoadedMsg is a finished load, already encoded by its command.
type imageLoadedMsg struct {
	post, src   string
	id, evicted int
	prep        images.Prepared
	upload      string
	err         error
}

type imageReadyMsg struct{ post, src string }

func postKey(uuid, link, title string) string {
	return uuid + "\x00" + link + "\x00" + title
}

// imageMaxRows is the configured image height, or the reader's height.
func (m *Model) imageMaxRows() int {
	if h := m.cfg.Reader.ImageMaxHeight; h > 0 {
		return h
	}
	return max(m.viewport.Height()-m.viewport.Style.GetVerticalFrameSize(), 1)
}

// pickSource chooses the smallest srcset candidate at least wantPx wide, so
// images stay sharp when the terminal scales them up, else the largest.
func pickSource(img reader.Image, wantPx int) string {
	best := reader.Candidate{}
	for _, c := range img.Srcset {
		switch {
		case best.URL == "":
			best = c
		case best.W < wantPx && c.W > best.W:
			best = c // still too small: take anything bigger
		case c.W >= wantPx && c.W < best.W:
			best = c // wide enough and smaller
		}
	}
	if best.URL == "" {
		return img.Src
	}
	return best.URL
}

// loadImages starts loading doc's new images. Ids are assigned here; all the
// slow work (fetch, decode, scale, encode) happens in the returned commands.
func (m *Model) loadImages(doc reader.Document) tea.Cmd {
	if !m.imgOn {
		return nil
	}
	var cmds []tea.Cmd
	fetcher, post, cell := m.imgFetcher, m.imgPost, m.cell
	maxCols, maxRows := m.contentWidth(), m.imageMaxRows()
	for _, img := range doc.Images {
		if _, known := m.imgs[img.Src]; known {
			continue
		}
		id, evicted := m.ids.Acquire(img.Src)
		m.imgs[img.Src] = &postImage{id: id}
		src, cssW := img.Src, img.Width
		fetchURL := pickSource(img, maxCols*cellWidth(cell))
		cmds = append(cmds, func() tea.Msg {
			msg := imageLoadedMsg{post: post, src: src, id: id, evicted: evicted}
			data, err := fetcher.Load(fetchURL)
			if err != nil {
				msg.err = err
				return msg
			}
			if msg.prep, err = images.Prepare(data, cssW, maxCols, maxRows, cell); err != nil {
				msg.err = err
				return msg
			}
			if msg.upload, err = images.Upload(id, msg.prep); err != nil {
				msg.err = err
				return msg
			}
			msg.prep.Img = nil // only the encoded upload is needed from here on
			return msg
		})
	}
	return tea.Batch(cmds...)
}

// imageLoaded records a finished load and, once on screen, sends the upload.
func (m *Model) imageLoaded(msg imageLoadedMsg) tea.Cmd {
	pi, ok := m.imgs[msg.src]
	if msg.post != m.imgPost || !ok {
		return nil
	}
	pi.loaded = true
	var del string
	if msg.evicted != 0 {
		del = images.Delete(msg.evicted)
	}
	if msg.err != nil {
		log.Printf("loading image %s: %v", msg.src, msg.err)
		pi.failed = true
		m.rerenderReader()
		return rawCmd(del)
	}
	pi.prep, pi.id, pi.upload = msg.prep, msg.id, msg.upload
	if !m.ready {
		return nil // the upload must happen on the alt screen, after the first frame
	}
	post, src := msg.post, msg.src
	return tea.Sequence(tea.Raw(del+msg.upload), func() tea.Msg { return imageReadyMsg{post: post, src: src} })
}

func (m *Model) imageReady(msg imageReadyMsg) {
	pi, ok := m.imgs[msg.src]
	if msg.post != m.imgPost || !ok {
		return
	}
	pi.uploaded = true
	m.rerenderReader()
}

// rerenderReader re-renders the open post, keeping the line being read in
// place when images above it change height.
func (m *Model) rerenderReader() {
	if m.screen != screenReader {
		return
	}
	old := m.doc
	offset := m.viewport.YOffset()
	m.renderReader()
	for i, img := range m.doc.Images {
		if i < len(old.Images) && old.Images[i].Src == img.Src && old.Images[i].Line < offset {
			offset += img.Height - old.Images[i].Height
		}
	}
	m.viewport.SetYOffset(offset)
}

// resizeImages re-fits uploaded images to the reader width, re-placing them
// in the terminal without re-sending pixels.
func (m *Model) resizeImages() tea.Cmd {
	var b strings.Builder
	width := m.contentWidth()
	for _, pi := range m.imgs {
		if !pi.uploaded {
			continue
		}
		cols, rows := images.Fit(pi.prep.CSSW, pi.prep.CSSH, width, m.imageMaxRows(), m.cell)
		if cols == pi.prep.Cols && rows == pi.prep.Rows {
			continue
		}
		pi.prep.Cols, pi.prep.Rows = cols, rows
		b.WriteString(images.Place(pi.id, cols, rows))
	}
	return rawCmd(b.String())
}

// reuploadImages re-sends the open post's images, e.g. after a suspend left
// and re-entered the alt screen, which drops the terminal's image store.
func (m *Model) reuploadImages() tea.Cmd {
	var b strings.Builder
	for _, pi := range m.imgs {
		if !pi.uploaded {
			continue
		}
		b.WriteString(pi.upload)
		b.WriteString(images.Place(pi.id, pi.prep.Cols, pi.prep.Rows)) // the size may have changed since
	}
	return rawCmd(b.String())
}

// rawCmd writes s straight to the terminal, or does nothing when s is empty.
func rawCmd(s string) tea.Cmd {
	if s == "" {
		return nil
	}
	return tea.Raw(s)
}

// readyRows returns the placeholder rows for every uploaded image.
func (m *Model) readyRows() (ready map[string][]string, failed map[string]bool) {
	ready, failed = map[string][]string{}, map[string]bool{}
	for src, pi := range m.imgs {
		switch {
		case pi.failed:
			failed[src] = true
		case pi.uploaded:
			ready[src] = pi.placeholders()
		}
	}
	return ready, failed
}

// placeholders returns the image's placeholder rows, rebuilt only on resize.
func (pi *postImage) placeholders() []string {
	if pi.rows == nil || pi.rowCols != pi.prep.Cols || pi.rowRows != pi.prep.Rows {
		pi.rows = images.Placeholders(pi.id, pi.prep.Cols, pi.prep.Rows)
		pi.rowCols, pi.rowRows = pi.prep.Cols, pi.prep.Rows
	}
	return pi.rows
}

// cellWidth is the cell width in device pixels, assuming 8 when unknown.
func cellWidth(c images.Cell) int {
	if c.W > 0 {
		return c.W
	}
	return 8
}

func (m *Model) refreshCellSize() {
	if m.imgOn {
		m.cell = images.CellSize(int(os.Stdout.Fd()))
	}
}
