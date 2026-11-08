package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/imgsession"
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
	m.refresh.Start([]string{sampleURL(1)})
	m, _ = update(m, feedLoadedMsg{feed: rss.Feed{URL: "https://Beta.example/feed", Title: "Beta.example", Err: timeoutErr()}})
	if got := toastTexts(m); len(got) != 1 || got[0] != "Beta: timed out" {
		t.Errorf("toasts = %q, want [Beta: timed out]", got)
	}
	if len(feedAt(m, 1).Posts) != 4 {
		t.Error("a failed refresh must keep the cached posts")
	}
}

func TestRefreshFailures_ToastBurst(t *testing.T) {
	buf := captureLog(t)
	m := newTestModel(t)
	noTicks(&m)
	for i := range 11 {
		m.lib.Replace(rss.Feed{Title: fmt.Sprintf("F%d", i), URL: fmt.Sprintf("https://f%d.example", i)})
	}
	m.refresh.Start(m.lib.URLs()[3:])
	for i := range 11 {
		f := rss.Feed{URL: fmt.Sprintf("https://f%d.example", i), Err: timeoutErr()}
		m, _ = update(m, feedLoadedMsg{feed: f})
	}
	if n := len(m.toasts.items); n > 3 {
		t.Errorf("%d toasts visible, want at most 3", n)
	}
	if n := strings.Count(buf.String(), "timed out"); n != 11 {
		t.Errorf("log has %d failures, want 11", n)
	}
}

func TestTrackingError_Toast(t *testing.T) {
	m, db := newTestModelDB(t)
	noTicks(&m)
	_ = db.Close()
	m, _ = keys(m, "l", "x")
	if got := toastTexts(m); len(got) != 1 || got[0] != "couldn't save read status" {
		t.Errorf("toasts = %q", got)
	}
}

func TestOpenLinkError_Toast(t *testing.T) {
	m := newTestModel(t)
	noTicks(&m)
	m.openURL = func(string) error { return errors.New("no browser") }
	m, _ = keys(m, "l", "o")
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
		m, _ = update(m, imageLoadedMsg{imgsession.Result{Post: sessionKey(m.page.Ref()), Src: src, Err: errors.New("404")}})
	}
	if got := toastTexts(m); len(got) != 1 || got[0] != "2 images in this post couldn't be loaded" {
		t.Errorf("toasts = %q", got)
	}
}

func TestToastsRenderedTopRight(t *testing.T) {
	m := newTestModel(t)
	noTicks(&m)
	m.notify("something broke")
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	top, row := lines[0], strings.TrimRight(lines[1], " │")
	if !strings.HasSuffix(top, "╮") || !strings.HasSuffix(row, "something broke") {
		t.Errorf("toast should sit at the top right:\n%s\n%s", top, lines[1])
	}
}

func TestToastOverReaderKeepsPlaceholders(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	noTicks(&m)
	m, _ = loadAndShow(t, m, postWithImage(srv.URL+"/a.png"))
	m.notify("boom")
	view := m.View().Content
	if !strings.Contains(view, placeholder) || !strings.Contains(view, "38;5;1") {
		t.Error("composing a toast over the reader lost the image placeholders")
	}
}
