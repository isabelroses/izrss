package ui

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/isabelroses/izrss/internal/images"
	"github.com/isabelroses/izrss/internal/imgsession"
	"github.com/isabelroses/izrss/internal/rss"
)

// imageLoadedMsg is a finished load, already encoded by its command.
type imageLoadedMsg struct{ imgsession.Result }

// imageReadyMsg says an image's upload was written.
type imageReadyMsg struct{ post, src string }

// sessionKey names the open post to the image session.
func sessionKey(ref rss.PostRef) string { return ref.FeedURL + "\x00" + ref.Key }

// imageMaxRows is the configured image height, or the reader's height.
func (m *Model) imageMaxRows() int {
	if h := m.cfg.Reader.ImageMaxHeight; h > 0 {
		return h
	}
	return max(m.viewport.Height()-m.viewport.Style.GetVerticalFrameSize(), 1)
}

// imageLayout is the room the open post's images are fitted to.
func (m *Model) imageLayout() imgsession.Layout {
	return imgsession.Layout{Cols: m.contentWidth(), Rows: m.imageMaxRows(), Cell: m.cell}
}

// imageJobs turns the session's loads into commands.
func imageJobs(jobs []imgsession.Job) tea.Cmd {
	cmds := make([]tea.Cmd, len(jobs))
	for i, job := range jobs {
		cmds[i] = func() tea.Msg { return imageLoadedMsg{job.Run()} }
	}
	return tea.Batch(cmds...)
}

// imageOutcome acts on what the session asks: write its escapes (and confirm
// the upload afterwards), render again, toast failures.
func (m *Model) imageOutcome(o imgsession.Outcome) tea.Cmd {
	var cmds []tea.Cmd
	if o.Write != "" {
		write := tea.Raw(o.Write)
		if o.Confirm {
			post, src := o.Post, o.Src
			cmds = append(cmds, tea.Sequence(write, func() tea.Msg { return imageReadyMsg{post: post, src: src} }))
		} else {
			cmds = append(cmds, write)
		}
	}
	if o.Rerender {
		m.rerenderReader()
	}
	switch {
	case o.Failed == 1:
		cmds = append(cmds, m.notify("1 image in this post couldn't be loaded"))
	case o.Failed > 1:
		cmds = append(cmds, m.notify(fmt.Sprintf("%d images in this post couldn't be loaded", o.Failed)))
	}
	return tea.Batch(cmds...)
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

// rawCmd writes s straight to the terminal, or does nothing when s is empty.
func rawCmd(s string) tea.Cmd {
	if s == "" {
		return nil
	}
	return tea.Raw(s)
}

func (m *Model) refreshCellSize() {
	if m.images.Enabled() {
		m.cell = images.CellSize(int(os.Stdout.Fd()))
	}
}
