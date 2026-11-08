package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/rss"
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
	return toastTheme{text: lipgloss.Color("#ffffff"), subtle: lipgloss.Color("#a6adc8"), err: lipgloss.Color("#f38ba8")}
}

func TestToasts_AddAndExpire(t *testing.T) {
	var ts toasts
	recordTicks(&ts)
	cmd := ts.add("boom")
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
	ts.add("x")
	if len(*ds) != 1 || (*ds)[0] != 6500*time.Millisecond || toastTTL != 6500*time.Millisecond {
		t.Errorf("toast should last 6.5s, ticks %v", *ds)
	}
}

func TestToasts_MergeDuplicates(t *testing.T) {
	var ts toasts
	recordTicks(&ts)
	first := ts.add("same")
	second := ts.add("same")
	if len(ts.items) != 1 || !strings.Contains(ansi.Strip(toastView(ts, 80)), "×2") {
		t.Fatalf("duplicates should merge with a count: %q", ansi.Strip(toastView(ts, 80)))
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
		ts.add(s)
	}
	lines := toastLines(toastView(ts, 80))
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
	ts.add(strings.Repeat("long ", 60))
	v := toastView(ts, 40)
	for l := range strings.SplitSeq(v, "\n") {
		if w := ansi.StringWidth(l); w > 40 {
			t.Errorf("toast %d wide: %q", w, ansi.Strip(l))
		}
	}
	if lines := toastLines(v); len(lines) != 1 || !strings.Contains(lines[0], "…") {
		t.Errorf("long toast should be cut to one line with …: %q", lines)
	}
}

func TestToasts_Prefix(t *testing.T) {
	var ts toasts
	recordTicks(&ts)
	ts.add("e")
	v := ansi.Strip(toastView(ts, 80))
	if !strings.Contains(v, "✗ e") {
		t.Errorf("prefixes missing: %q", v)
	}
	if first := strings.Split(v, "\n")[0]; !strings.HasPrefix(first, "╭") || !strings.HasSuffix(first, "╮") {
		t.Errorf("toasts should sit in a rounded box like the help overlay: %q", v)
	}
	ts.add("f")
	if boxes := ts.view(80, testToastTheme()); len(boxes) != 2 || strings.Count(ansi.Strip(boxes[0]), "╭") != 1 {
		t.Errorf("each toast should get its own box: %q", boxes)
	}
	if len((toasts{}).view(80, testToastTheme())) != 0 {
		t.Error("no toasts should render nothing")
	}
}

func TestNotify_StripsControlChars(t *testing.T) {
	m := newTestModel(t)
	m.notify("feed: bad \x1b]52;c;aGk=\x07\nthing")
	got := m.toasts.items[0].text
	if strings.ContainsAny(got, "\x1b\x07\n") {
		t.Fatalf("control characters reached the toast: %q", got)
	}
}

// A fetch failure is put in a few words from its type, not its wording.
func TestShortReason(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{fmt.Errorf("fetching u: %w", &rss.StatusError{Code: 404, Status: "404 Not Found"}), "HTTP 404"},
		{fmt.Errorf("parsing u: %w", &rss.ParseError{Err: errors.New("EOF")}), "couldn't parse feed"},
		{fmt.Errorf("fetching u: %w", &net.DNSError{Err: "no such host", Name: "u", IsNotFound: true}), "couldn't find host"},
		{fmt.Errorf("fetching u: %w", context.DeadlineExceeded), "timed out"},
		{errors.New("reading u: feed is larger than 32 MiB"), "feed is larger than 32 MiB"},
	}
	for _, tt := range tests {
		if got := shortReason(tt.err); got != tt.want {
			t.Errorf("shortReason(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

// toastLines is the toasts' own rows of a view, without the box around them.
func toastLines(view string) []string {
	var out []string
	for l := range strings.SplitSeq(ansi.Strip(view), "\n") {
		if strings.Contains(l, "✗") {
			out = append(out, l)
		}
	}
	return out
}

// toastView is every toast box, stacked.
func toastView(ts toasts, width int) string {
	return strings.Join(ts.view(width, testToastTheme()), "\n")
}
