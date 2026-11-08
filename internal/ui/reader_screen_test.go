package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/rss"
)

func TestReader_HeaderLine(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "enter")
	if !m.page.Shown() {
		t.Fatal("enter on a post should open the reader")
	}
	lines := strings.Split(m.View().Content, "\n")
	head := ansi.Strip(lines[0])
	for _, want := range []string{"Alpha post 0", "Alpha", "04/11/2026", "%"} {
		if !strings.Contains(head, want) {
			t.Errorf("header %q lacks %q", head, want)
		}
	}
	if ansi.StringWidth(lines[0]) > 120 || len(lines) != 40 {
		t.Errorf("reader view: %d lines, header %d wide", len(lines), ansi.StringWidth(lines[0]))
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "Alpha body 0") {
		t.Error("reader should show the post body")
	}
}

func TestReader_BackPreservesSelection(t *testing.T) {
	for _, back := range []string{"esc", "q", "h", "left"} {
		m, _ := keys(newTestModel(t), "j", "l", "j", "j", "j") // Beta, post 3
		m, _ = keys(m, "enter")
		m, msgs := keys(m, back)
		if quits(msgs) {
			t.Fatalf("%s in the reader must not quit", back)
		}
		if m.page.Shown() || m.main.focus != panePosts || selectedTitle(m.main.feeds) != "Beta" || selectedTitle(m.main.posts) != "Beta post 3" {
			t.Errorf("%s: reader %v focus %v feed %q post %q", back, m.page.Shown(), m.main.focus, selectedTitle(m.main.feeds), selectedTitle(m.main.posts))
		}
	}
}

func TestReader_ToggleReadStays(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "enter")
	m.page.viewport.SetHeight(1) // keep the threshold from auto-marking
	before := isRead(m, 0, 0)
	m, _ = keys(m, "x")
	if !m.page.Shown() || isRead(m, 0, 0) == before {
		t.Errorf("x should toggle read and stay: reader %v read %v", m.page.Shown(), isRead(m, 0, 0))
	}
}

func TestReader_ThresholdMarksByKey(t *testing.T) {
	m, _ := keys(newTestModel(t), "k", "l", "j", "j") // All posts row 2 = Gamma post 0
	if selectedTitle(m.main.posts) != "Gamma post 0" {
		t.Fatalf("selected %q", selectedTitle(m.main.posts))
	}
	m, _ = keys(m, "enter") // short post: fully visible, past any threshold
	m, _ = update(m, tea.KeyPressMsg{Code: tea.KeyDown})
	if !isRead(m, 2, 0) {
		t.Error("Gamma post 0 should be marked read")
	}
	if isRead(m, 0, 0) || isRead(m, 1, 0) {
		t.Error("other feeds' posts were marked read (index bug)")
	}
}

func TestReader_ViewportHeight(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "enter")
	if h := m.page.viewport.Height(); h != 37 {
		t.Errorf("viewport height %d, want 37", h)
	}
	m, _ = update(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m, _ = keys(m, "esc", "enter")
	if h := m.page.viewport.Height(); h != 27 {
		t.Errorf("after resize and reopen: %d, want 27", h)
	}
}

func TestOpenReader_RendersHTML(t *testing.T) {
	m := newTestModel(t)
	feedAt(m, 0).Posts[0].Content = "<h2>Hi</h2><p>there</p>"
	m.openReader(feedAt(m, 0).Posts[0].Ref(feedAt(m, 0).URL))
	view := ansi.Strip(m.page.viewport.View())
	if !strings.Contains(view, "## Hi") || !strings.Contains(view, "there") {
		t.Errorf("reader should render HTML:\n%s", view)
	}
}

// The page has a margin each side and the document its own inside it; the
// text column inside the page margins is what reader.size sets.
func TestReader_SideMargin(t *testing.T) {
	m := newTestModel(t)
	m.cfg.Reader.Width = config.Width{Percent: 100}
	long := strings.Repeat("word ", 60)
	feedAt(m, 0).Posts[0].Content = "<p>" + long + "</p>"
	m, _ = keys(m, "l", "enter")
	body := strings.Split(m.View().Content, "\n")[1:39]
	widest := 0
	for i, line := range body {
		plain := ansi.Strip(line)
		if strings.TrimSpace(plain) == "" {
			continue
		}
		if !strings.HasPrefix(plain, "    ") || strings.HasPrefix(plain, "     ") {
			t.Errorf("line %d should have a four-column left margin: %q", i, plain)
		}
		widest = max(widest, len(strings.TrimRight(plain, " ")))
	}
	if widest > 116 || widest < 108 {
		t.Errorf("text should fill the width inside both margins, widest %d", widest)
	}
	if got := m.page.opts().Width; got != 116 {
		t.Errorf("full size text width = %d, want 116", got)
	}
}

// The post sits inside the page margin and the document's own margin, and
// images are fitted to the text column inside both.
func TestReader_DocumentMargin(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "enter")
	var body string
	for line := range strings.SplitSeq(ansi.Strip(m.View().Content), "\n") {
		if strings.Contains(line, "Alpha body 0") {
			body = line
		}
	}
	if want := strings.Repeat(" ", readerMargin+docMargin) + "Alpha body 0"; !strings.HasPrefix(body, want) || strings.HasPrefix(body, want[:len(want)-len("Alpha body 0")]+" ") {
		t.Errorf("body line %q should start %q", body, want)
	}
	if got, want := m.page.contentWidth(), m.page.textWidth()-2*docMargin; got != want {
		t.Errorf("images fit %d columns, want %d", got, want)
	}
}

// A blank row separates the end of the article from the hint line, and the
// reader still fits short terminals.
func TestReader_GapAboveHintLine(t *testing.T) {
	var b strings.Builder
	for i := range 100 {
		fmt.Fprintf(&b, "<p>line %d</p>", i)
	}
	m, _ := openPost(newTestModel(t), rss.Post{UUID: "long", Title: "long", Content: b.String()})
	m.page.viewport.GotoBottom()
	if m.page.viewport.ScrollPercent() != 1 {
		t.Fatalf("scroll percent %v at the bottom, want 1", m.page.viewport.ScrollPercent())
	}
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	if len(lines) != 40 || !strings.Contains(lines[len(lines)-3], "line 99") || strings.TrimSpace(lines[len(lines)-2]) != "" {
		t.Errorf("want the last line, a blank row, then the hint line; got %q", lines[len(lines)-4:])
	}
	for _, h := range []int{7, 8} {
		m, _ = update(m, tea.WindowSizeMsg{Width: 80, Height: h})
		if n := strings.Count(m.View().Content, "\n") + 1; n != h {
			t.Errorf("height %d: %d lines", h, n)
		}
	}
}
