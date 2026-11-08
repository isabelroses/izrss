package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// recordTicks swaps tea.Tick for a recorder so tests can fire expiries.
func recordTicks(t *toasts) *[]time.Duration {
	var ds []time.Duration
	t.tick = func(d time.Duration, fn func(time.Time) tea.Msg) tea.Cmd {
		ds = append(ds, d)
		return func() tea.Msg { return fn(time.Time{}) }
	}
	return &ds
}

func testToastTheme() toastTheme {
	return toastTheme{text: lipgloss.Color("#ffffff"), bg: lipgloss.Color("#313244"), err: lipgloss.Color("#f38ba8"), warn: lipgloss.Color("#f9e2af")}
}

func TestToasts_AddAndExpire(t *testing.T) {
	var ts toasts
	recordTicks(&ts)
	cmd := ts.add(toastError, "boom")
	if len(ts.items) != 1 {
		t.Fatalf("items = %d", len(ts.items))
	}
	msg, ok := cmd().(toastExpiredMsg)
	if !ok {
		t.Fatalf("cmd should yield toastExpiredMsg")
	}
	ts.expire(msg)
	if len(ts.items) != 0 {
		t.Errorf("toast should be gone after expiry")
	}
}

func TestToasts_TTL(t *testing.T) {
	var ts toasts
	ds := recordTicks(&ts)
	ts.add(toastError, "x")
	if len(*ds) != 1 || (*ds)[0] != 3*time.Second || toastTTL != 3*time.Second {
		t.Errorf("toast should last 3s, ticks %v", *ds)
	}
}

func TestToasts_MergeDuplicates(t *testing.T) {
	var ts toasts
	recordTicks(&ts)
	first := ts.add(toastError, "same")
	second := ts.add(toastError, "same")
	if len(ts.items) != 1 || !strings.Contains(ansi.Strip(ts.view(80, testToastTheme())), "×2") {
		t.Fatalf("duplicates should merge with a count: %q", ansi.Strip(ts.view(80, testToastTheme())))
	}
	ts.expire(first().(toastExpiredMsg))
	if len(ts.items) != 1 {
		t.Error("the first timer is stale after a merge and must not remove the toast")
	}
	ts.expire(second().(toastExpiredMsg))
	if len(ts.items) != 0 {
		t.Error("the restarted timer should remove it")
	}
}

func TestToasts_Cap(t *testing.T) {
	var ts toasts
	recordTicks(&ts)
	for _, s := range []string{"a", "b", "c", "d", "e"} {
		ts.add(toastError, s)
	}
	lines := strings.Split(ansi.Strip(ts.view(80, testToastTheme())), "\n")
	if len(ts.items) != maxToasts || maxToasts != 3 || len(lines) != 3 {
		t.Fatalf("want 3 toasts, got %d: %q", len(ts.items), lines)
	}
	if !strings.Contains(lines[0], "e") || !strings.Contains(lines[2], "c") {
		t.Errorf("newest first, oldest dropped: %q", lines)
	}
}

func TestToasts_ViewFitsWidth(t *testing.T) {
	var ts toasts
	recordTicks(&ts)
	ts.add(toastError, strings.Repeat("long ", 60))
	for l := range strings.SplitSeq(ts.view(40, testToastTheme()), "\n") {
		if w := ansi.StringWidth(l); w > 40 {
			t.Errorf("toast %d wide: %q", w, ansi.Strip(l))
		}
		if !strings.Contains(ansi.Strip(l), "…") {
			t.Errorf("long toast should be truncated with …: %q", ansi.Strip(l))
		}
	}
}

func TestToasts_Prefix(t *testing.T) {
	var ts toasts
	recordTicks(&ts)
	ts.add(toastError, "e")
	ts.add(toastWarn, "w")
	v := ansi.Strip(ts.view(80, testToastTheme()))
	if !strings.Contains(v, "✗ e") || !strings.Contains(v, "! w") {
		t.Errorf("prefixes missing: %q", v)
	}
	if (toasts{}).view(80, testToastTheme()) != "" {
		t.Error("no toasts should render nothing")
	}
}

func TestNotify_StripsControlChars(t *testing.T) {
	m := newTestModel(t)
	m.notify(toastError, "feed: bad \x1b]52;c;aGk=\x07\nthing")
	got := m.toasts.items[0].text
	if strings.ContainsAny(got, "\x1b\x07\n") {
		t.Fatalf("control characters reached the toast: %q", got)
	}
}
