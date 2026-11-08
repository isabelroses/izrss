package reader

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var update = flag.Bool("update", false, "rewrite golden files")

func testTheme() Theme {
	return Theme{
		Text:      lipgloss.Color("#cdd6f4"),
		Subtle:    lipgloss.Color("#a6adc8"),
		Heading:   lipgloss.Color("#74c7ec"),
		Link:      lipgloss.Color("#74c7ec"),
		Code:      lipgloss.Color("#a6adc8"),
		Quote:     lipgloss.Color("#a6adc8"),
		CodeTheme: "catppuccin-mocha",
	}
}

func opts(width int) Options {
	return Options{Width: width, Theme: testTheme(), BaseURL: "https://example.com/posts/1"}
}

func fixtures(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob("testdata/*.html")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	out := make(map[string]string, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out[strings.TrimSuffix(filepath.Base(p), ".html")] = string(b)
	}
	return out
}

func TestGolden(t *testing.T) {
	for name, src := range fixtures(t) {
		for _, width := range []int{40, 80} {
			t.Run(name+"/"+itoa(width), func(t *testing.T) {
				got := ansi.Strip(Render(src, opts(width)).String())
				path := filepath.Join("testdata", name+"."+itoa(width)+".golden")
				if *update {
					if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
						t.Fatal(err)
					}
					return
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("missing golden (run with -update): %v", err)
				}
				if got != string(want) {
					t.Errorf("mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
				}
			})
		}
	}
}

func TestWidth_NeverExceeded(t *testing.T) {
	for name, src := range fixtures(t) {
		for _, width := range []int{20, 40, 80} {
			doc := Render(src, opts(width))
			for i, line := range doc.Lines {
				if w := ansi.StringWidth(line); w > width {
					t.Errorf("%s@%d line %d is %d wide: %q", name, width, i, w, ansi.Strip(line))
				}
			}
		}
	}
}

func TestLinks_EmitHyperlink(t *testing.T) {
	out := Render(`<p><a href="https://x.dev/a">read</a></p>`, opts(80)).String()

	if !strings.Contains(out, ansi.SetHyperlink("https://x.dev/a")) {
		t.Errorf("expected OSC 8 hyperlink to https://x.dev/a in %q", out)
	}
	if visible := ansi.Strip(out); strings.Contains(visible, "https://x.dev/a") || !strings.Contains(visible, "read") {
		t.Errorf("visible text should be the link text only, got %q", visible)
	}
}

func TestLinks_RelativeHref(t *testing.T) {
	o := opts(80)
	o.BaseURL = "https://x.dev/posts/1"
	out := Render(`<p><a href="/b">b</a></p>`, o).String()

	if !strings.Contains(out, ansi.SetHyperlink("https://x.dev/b")) {
		t.Errorf("expected relative href resolved to https://x.dev/b in %q", out)
	}
}

func TestEmpty(t *testing.T) {
	if doc := Render("", opts(80)); len(doc.Lines) != 0 {
		t.Errorf("expected no lines, got %q", doc.Lines)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
