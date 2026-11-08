package reader

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

const goSrc = "func main() {}"

// unbox strips a code box line back to its text: styles, the one-cell
// padding and the fill to the width.
func unbox(line string) string {
	return strings.TrimRight(strings.TrimPrefix(ansi.Strip(line), " "), " ")
}

func unboxAll(doc Document) []string {
	out := make([]string, len(doc.Lines))
	for i, l := range doc.Lines {
		out[i] = unbox(l)
	}
	return out
}

func TestCode_Highlighted(t *testing.T) {
	out := Render(`<pre><code class="language-go">`+goSrc+`</code></pre>`, opts(80)).String()
	if unbox(out) != goSrc {
		t.Errorf("stripped code changed: %q", unbox(out))
	}
	plainCode := Render(`<pre><code class="language-text">`+goSrc+`</code></pre>`, opts(80)).String()
	if out == plainCode {
		t.Errorf("go code should be highlighted differently from plain text")
	}
}

func TestCode_UnknownLanguagePlain(t *testing.T) {
	out := Render(`<pre><code class="language-nope">`+goSrc+`</code></pre>`, opts(80)).String()
	if unbox(out) != goSrc {
		t.Errorf("got %q", unbox(out))
	}
}

func TestCode_UnknownStyleFallsBack(t *testing.T) {
	o := opts(80)
	o.Theme.CodeTheme = "does-not-exist"
	out := Render(`<pre><code class="language-go">`+goSrc+`</code></pre>`, o).String()
	if unbox(out) != goSrc || !strings.Contains(out, "\x1b[") {
		t.Errorf("expected highlighted output with fallback style, got %q", out)
	}
}

func TestCode_HighlightedLinesTruncate(t *testing.T) {
	src := "var x = \"" + strings.Repeat("a", 100) + "\""
	doc := Render(`<pre><code class="language-go">`+src+`</code></pre>`, opts(30))
	if len(doc.Lines) != 1 || ansi.StringWidth(doc.Lines[0]) != 30 || !strings.HasSuffix(unbox(doc.Lines[0]), "…") {
		t.Errorf("highlighted long line should truncate to 30 with …: %q", plain(doc))
	}
}

func TestTable_Grid(t *testing.T) {
	lines := plain(Render(`<table><tr><th>A</th><th>Bee</th></tr><tr><td>1</td><td>2</td></tr></table>`, opts(80)))
	want := []string{"A │ Bee", "──┼────", "1 │ 2  "}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestTable_FitsWidth(t *testing.T) {
	doc := Render(mustRead(t, "testdata/table-wide.html"), opts(40))
	sawEllipsis := false
	for _, l := range doc.Lines {
		if w := ansi.StringWidth(l); w > 40 {
			t.Errorf("line %d wide: %q", w, ansi.Strip(l))
		}
		sawEllipsis = sawEllipsis || strings.Contains(ansi.Strip(l), "…")
	}
	if !sawEllipsis {
		t.Errorf("shrunk cells should end with …")
	}
}

func TestTable_Ragged(t *testing.T) {
	lines := plain(Render(`<table><tr><th>a</th><th>b</th><th>c</th></tr><tr><td colspan="2">x</td></tr><tr><td rowspan="3">y</td><td>z</td><td>w</td><td>extra</td></tr></table>`, opts(80)))
	if len(lines) != 4 {
		t.Fatalf("expected header, rule and 2 rows, got %q", lines)
	}
	for _, l := range lines {
		if strings.Count(l, "│")+strings.Count(l, "┼") != 3 {
			t.Errorf("every row should have 4 columns (the widest row): %q", l)
		}
	}
}
