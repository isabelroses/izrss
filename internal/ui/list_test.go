package ui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func testListTheme() listTheme {
	return newListTheme(lipgloss.Color("#cdd6f4"), lipgloss.Color("#a6adc8"), lipgloss.Color("#74c7ec"), lipgloss.Color("#1e1e2e"), lipgloss.Color("#313244"))
}

func items(n int) []item {
	out := make([]item, n)
	for i := range out {
		out[i] = item{key: fmt.Sprint(i), title: fmt.Sprintf("item %d", i), right: "04 Nov"}
	}
	return out
}

func TestList_MoveAndScroll(t *testing.T) {
	l := list{height: 5}
	l.setItems(items(20), item{})
	l.move(7)
	if l.cursor != 7 || l.offset != 3 {
		t.Errorf("cursor %d offset %d, want 7 and 3", l.cursor, l.offset)
	}
	l.move(-10)
	if l.cursor != 0 || l.offset != 0 {
		t.Errorf("cursor %d offset %d, want 0 and 0", l.cursor, l.offset)
	}
	l.move(100)
	if l.cursor != 19 || l.offset != 15 {
		t.Errorf("cursor %d offset %d, want 19 and 15", l.cursor, l.offset)
	}
}

func TestList_SetItemsKeepsKey(t *testing.T) {
	l := list{height: 10}
	l.setItems(items(6), item{})
	l.move(2)
	moved := append([]item{{key: "x"}, {key: "y"}, {key: "z"}}, items(6)...)
	l.setItems(moved, item{key: "2"})
	if sel, _ := l.selected(); sel.key != "2" || l.cursor != 5 {
		t.Errorf("cursor should follow key 2 to row 5, got row %d key %q", l.cursor, sel.key)
	}
	l.setItems(items(3), item{key: "gone"})
	if l.cursor != 2 {
		t.Errorf("missing key should clamp the cursor, got %d", l.cursor)
	}
}

func TestList_TinyAndWide(t *testing.T) {
	its := []item{
		{key: "a", title: "日本語のとても長いフィードのタイトルです", right: "12", marker: "•", strong: true},
		{key: "b", title: "emoji 🦀🦀🦀🦀🦀🦀🦀🦀 feed", right: "a very long right column"},
		{key: "c", title: strings.Repeat("x", 300), right: "·"},
	}
	for _, h := range []int{0, 1, 3} {
		for _, w := range []int{1, 5, 20, 80} {
			l := list{height: h}
			l.setItems(its, item{})
			for _, focused := range []bool{true, false} {
				lines := l.view(w, focused, testListTheme())
				if len(lines) != h {
					t.Errorf("w=%d h=%d: %d lines", w, h, len(lines))
				}
				for _, line := range lines {
					if sw := ansi.StringWidth(line); sw > w {
						t.Errorf("w=%d h=%d: line %d wide: %q", w, h, sw, ansi.Strip(line))
					}
				}
			}
		}
	}
}

func TestList_Empty(t *testing.T) {
	l := list{height: 4}
	if _, ok := l.selected(); ok {
		t.Error("empty list has no selection")
	}
	l.move(3)
	if lines := l.view(10, true, testListTheme()); len(lines) != 4 {
		t.Errorf("want 4 blank lines, got %d", len(lines))
	}
}

func TestList_RowLayout(t *testing.T) {
	l := list{height: 2}
	l.setItems([]item{{key: "a", title: "Title", right: "12", marker: "•"}, {key: "b", title: "Feed", right: "3"}}, item{})
	lines := l.view(20, false, testListTheme())
	if got := ansi.Strip(lines[0]); got != " • Title         12 " {
		t.Errorf("row with marker: %q", got)
	}
	if got := ansi.Strip(lines[1]); got != " Feed             3 " {
		t.Errorf("row without marker has no marker column: %q", got)
	}
}

// The focused pane's selection is an accent bar; the other pane's selection
// stays visible as a muted bar.
func TestList_SelectionBars(t *testing.T) {
	th := testListTheme()
	l := list{height: 2}
	l.setItems(items(2), item{})
	focused := l.view(20, true, th)
	if !strings.HasPrefix(focused[0], th.sel) {
		t.Errorf("focused selection should open with the accent bar: %q", focused[0])
	}
	if strings.Contains(focused[1], th.sel) || strings.Contains(focused[1], th.selDim) {
		t.Errorf("other rows have no bar: %q", focused[1])
	}
	if dim := l.view(20, false, th); !strings.HasPrefix(dim[0], th.selDim) {
		t.Errorf("unfocused selection should use the muted bar: %q", dim[0])
	}
	for _, line := range append(focused, l.view(20, false, th)...) {
		if w := ansi.StringWidth(line); w != 20 {
			t.Errorf("row is %d wide, want 20", w)
		}
	}
}

func BenchmarkListView(b *testing.B) {
	l := list{height: 40}
	l.setItems(items(400), item{})
	l.move(200)
	b.ReportAllocs()
	for b.Loop() {
		_ = l.view(80, true, testListTheme())
	}
}
