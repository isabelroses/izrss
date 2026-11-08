package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/rss"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

// noTicks keeps toast timers from sleeping inside the synchronous test runner.
func noTicks(m *Model) {
	recordTicks(&m.toasts)
}

func toastTexts(m Model) []string {
	var out []string
	for _, it := range m.toasts.items {
		out = append(out, it.text)
	}
	return out
}

func timeoutErr() error {
	return fmt.Errorf("fetching https://Beta.example/feed: %w", context.DeadlineExceeded)
}

func TestFeedError_Toast(t *testing.T) {
	m := newTestModel(t)
	noTicks(&m)
	m, _ = update(m, refreshStartedMsg{total: 1})
	m, _ = update(m, feedLoadedMsg{feed: rss.Feed{URL: "https://Beta.example/feed", Title: "Beta.example", Err: timeoutErr()}, index: 1})
	if got := toastTexts(m); len(got) != 1 || got[0] != "Beta: timed out" {
		t.Errorf("toasts = %q, want [Beta: timed out]", got)
	}
	if len(m.feeds[1].Posts) != 4 {
		t.Error("a failed refresh must keep the cached posts")
	}
}

func TestRefreshFailures_ToastBurst(t *testing.T) {
	buf := captureLog(t)
	m := newTestModel(t)
	noTicks(&m)
	for i := range 11 {
		m.feeds = append(m.feeds, rss.Feed{Title: fmt.Sprintf("F%d", i), URL: fmt.Sprintf("https://f%d.example", i)})
	}
	m, _ = update(m, refreshStartedMsg{total: 11})
	for i := range 11 {
		f := rss.Feed{URL: fmt.Sprintf("https://f%d.example", i), Err: timeoutErr()}
		m, _ = update(m, feedLoadedMsg{feed: f, index: 3 + i})
	}
	if n := len(m.toasts.items); n > 3 {
		t.Errorf("%d toasts visible, want at most 3", n)
	}
	if n := strings.Count(buf.String(), "timed out"); n != 11 {
		t.Errorf("log has %d failures, want 11", n)
	}
}

func TestTrackingError_Toast(t *testing.T) {
	m := newTestModel(t)
	noTicks(&m)
	_ = m.db.Close()
	m, msgs := keys(m, "l", "x")
	m = applyNotifies(t, m, msgs)
	if got := toastTexts(m); len(got) != 1 || got[0] != "couldn't save read status" {
		t.Errorf("toasts = %q", got)
	}
}

func TestOpenLinkError_Toast(t *testing.T) {
	m := newTestModel(t)
	noTicks(&m)
	m.openURL = func(string) error { return errors.New("no browser") }
	m, msgs := keys(m, "l", "o")
	m = applyNotifies(t, m, msgs)
	if got := toastTexts(m); len(got) != 1 || got[0] != "couldn't open link: no browser" {
		t.Errorf("toasts = %q", got)
	}
}

func TestImageFailures_OneToastPerPost(t *testing.T) {
	m := imageModel(t)
	noTicks(&m)
	p := rss.Post{UUID: "imgs", Link: "https://example.com/p", Content: `<img src="https://example.invalid/a.png"><img src="https://example.invalid/b.png">`}
	m, _ = openPost(m, p)
	for _, src := range []string{"https://example.invalid/a.png", "https://example.invalid/b.png"} {
		var msgs []tea.Msg
		m, msgs = update(m, imageLoadedMsg{post: m.imgPost, src: src, err: errors.New("404")})
		m = applyNotifies(t, m, msgs)
	}
	if got := toastTexts(m); len(got) != 1 || got[0] != "2 images in this post couldn't be loaded" {
		t.Errorf("toasts = %q", got)
	}
}

func TestConfigWarnings_ToastOnStart(t *testing.T) {
	m := *NewModel(config.Default(), testDB(t), nil, nil, []string{"reader.theme is no longer supported"})
	noTicks(&m)
	m = applyNotifies(t, m, run(m.warningsCmd()))
	if got := toastTexts(m); len(got) != 1 || got[0] != "reader.theme is no longer supported" || m.toasts.items[0].kind != toastWarn {
		t.Errorf("toasts = %q", got)
	}
}

func TestToastsRenderedTopRight(t *testing.T) {
	m := newTestModel(t)
	noTicks(&m)
	m, _ = update(m, notifyErr("something broke"))
	first := strings.TrimRight(ansi.Strip(strings.Split(m.View().Content, "\n")[0]), " ")
	if !strings.HasSuffix(first, "something broke") {
		t.Errorf("toast should sit at the top right: %q", first)
	}
}

func TestToastOverReaderKeepsPlaceholders(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	noTicks(&m)
	m, _ = loadAndShow(t, m, postWithImage(srv.URL+"/a.png"))
	m, _ = update(m, notifyErr("boom"))
	id := m.imgs[srv.URL+"/a.png"].id
	view := m.View().Content
	if !strings.Contains(view, placeholder) || !strings.Contains(view, fmt.Sprintf("38;5;%d", id)) {
		t.Error("composing a toast over the reader lost the image placeholders")
	}
}

// applyNotifies feeds any notifyMsg in msgs back through Update.
func applyNotifies(t *testing.T, m Model, msgs []tea.Msg) Model {
	t.Helper()
	for _, msg := range msgs {
		if n, ok := msg.(notifyMsg); ok {
			m, _ = update(m, n)
		}
	}
	return m
}
