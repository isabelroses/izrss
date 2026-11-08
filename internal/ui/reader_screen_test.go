package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestReader_HeaderLine(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "enter")
	if m.screen != screenReader {
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
		if m.screen != screenMain || m.focus != panePosts || selectedTitle(m.feedList) != "Beta" || selectedTitle(m.postList) != "Beta post 3" {
			t.Errorf("%s: screen %v focus %v feed %q post %q", back, m.screen, m.focus, selectedTitle(m.feedList), selectedTitle(m.postList))
		}
	}
}

func TestReader_ToggleReadStays(t *testing.T) {
	m, _ := keys(newTestModel(t), "l", "enter")
	m.viewport.SetHeight(1) // keep the threshold from auto-marking
	before := isRead(m, 0, 0)
	m, _ = keys(m, "x")
	if m.screen != screenReader || isRead(m, 0, 0) == before {
		t.Errorf("x should toggle read and stay: screen %v read %v", m.screen, isRead(m, 0, 0))
	}
}

func TestReader_ThresholdMarksByKey(t *testing.T) {
	m, _ := keys(newTestModel(t), "k", "l", "j", "j") // All posts row 2 = Gamma post 0
	if selectedTitle(m.postList) != "Gamma post 0" {
		t.Fatalf("selected %q", selectedTitle(m.postList))
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
	if h := m.viewport.Height(); h != 38 {
		t.Errorf("viewport height %d, want 38", h)
	}
	m, _ = update(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m, _ = keys(m, "esc", "enter")
	if h := m.viewport.Height(); h != 28 {
		t.Errorf("after resize and reopen: %d, want 28", h)
	}
}

func TestOpenReader_RendersHTML(t *testing.T) {
	m := newTestModel(t)
	m.feeds.All()[0].Posts[0].Content = "<h2>Hi</h2><p>there</p>"
	m.openReader(m.feeds.All()[0].Posts[0].Ref(m.feeds.All()[0].URL))
	view := ansi.Strip(m.viewport.View())
	if !strings.Contains(view, "## Hi") || !strings.Contains(view, "there") {
		t.Errorf("reader should render HTML:\n%s", view)
	}
}

// The page has a margin each side and the document its own inside it; the
// text column inside the page margins is what reader.size sets.
func TestReader_SideMargin(t *testing.T) {
	m := newTestModel(t)
	m.cfg.Reader.Size = "full"
	long := strings.Repeat("word ", 60)
	m.feeds.All()[0].Posts[0].Content = "<p>" + long + "</p>"
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
	if got := m.readerOpts().Width; got != 116 {
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
	if got, want := m.contentWidth(), m.textWidth()-2*docMargin; got != want {
		t.Errorf("images fit %d columns, want %d", got, want)
	}
}
