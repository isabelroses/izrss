package reader

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func plain(doc Document) []string {
	out := make([]string, len(doc.Lines))
	for i, l := range doc.Lines {
		out[i] = ansi.Strip(l)
	}
	return out
}

func TestList_HangingIndent(t *testing.T) {
	doc := Render(`<ul><li>one two three four five six seven eight nine ten</li></ul>`, opts(20))
	lines := plain(doc)
	if len(lines) < 2 {
		t.Fatalf("expected the item to wrap, got %q", lines)
	}
	if !strings.HasPrefix(lines[0], "• ") {
		t.Errorf("first line should start with a bullet: %q", lines[0])
	}
	for _, l := range lines[1:] {
		if !strings.HasPrefix(l, "  ") || strings.HasPrefix(l, "   ") {
			t.Errorf("continuation should be indented exactly to the text column: %q", l)
		}
	}
}

func TestList_NestedBullets(t *testing.T) {
	lines := plain(Render(`<ul><li>a<ul><li>b<ul><li>c</li></ul></li></ul></li></ul>`, opts(40)))
	want := []string{"• a", "  ◦ b", "    ▪ c"}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q, want %q", lines, want)
	}
}

func TestOrderedList_Start(t *testing.T) {
	lines := plain(Render(`<ol start="5"><li>a</li><li>b</li></ol>`, opts(40)))
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "5. ") || !strings.HasPrefix(lines[1], "6. ") {
		t.Errorf("got %q", lines)
	}
}

func TestOrderedList_AlignsNumbers(t *testing.T) {
	var b strings.Builder
	b.WriteString("<ol>")
	for range 10 {
		b.WriteString("<li>x</li>")
	}
	b.WriteString("</ol>")
	lines := plain(Render(b.String(), opts(40)))
	if lines[0] != " 1. x" || lines[9] != "10. x" {
		t.Errorf("numbers should right-align: %q / %q", lines[0], lines[9])
	}
}

func TestBlockquote_PrefixesEveryLine(t *testing.T) {
	lines := plain(Render(`<blockquote><p>one two three four five six seven eight</p><blockquote><p>inner</p></blockquote></blockquote>`, opts(20)))
	for _, l := range lines {
		if !strings.HasPrefix(l, "│") {
			t.Errorf("every quoted line needs the bar: %q", l)
		}
	}
	if last := lines[len(lines)-1]; !strings.HasPrefix(last, "│ │ ") {
		t.Errorf("nested quote should stack bars: %q", last)
	}
}

func TestRule_FullWidth(t *testing.T) {
	lines := plain(Render(`<p>a</p><hr><p>b</p>`, opts(30)))
	if lines[2] != strings.Repeat("─", 30) {
		t.Errorf("rule should span the width: %q", lines[2])
	}
}

func TestPre_PreservesSpacesAndTruncates(t *testing.T) {
	doc := Render("<pre>  two\n\ttab\n"+strings.Repeat("y", 50)+"</pre>", opts(20))
	lines := unboxAll(doc)
	if lines[0] != "  two" {
		t.Errorf("leading spaces lost: %q", lines[0])
	}
	if lines[1] != "    tab" {
		t.Errorf("tab should expand to 4 spaces: %q", lines[1])
	}
	if ansi.StringWidth(doc.Lines[2]) != 20 || !strings.HasSuffix(lines[2], "…") {
		t.Errorf("long code line should truncate with …: %q", lines[2])
	}
	if len(lines) != 3 {
		t.Errorf("code must not wrap: %q", lines)
	}
}

func TestFigcaption_AfterFigure(t *testing.T) {
	lines := plain(Render(`<figure><p>body</p><figcaption>caption</figcaption></figure>`, opts(40)))
	if lines[len(lines)-1] != "caption" {
		t.Errorf("caption should come last: %q", lines)
	}
}

// Some highlighters (e.g. isabelroses.com) wrap each code line in its own
// <div> with no newline characters; a browser breaks lines at those blocks.
func TestPre_BlockPerLine(t *testing.T) {
	src := `<pre><code><div><div><span>keys</span><span>:</span></div></div>` +
		`<div><div><span>  </span><span>-</span><span> a</span></div></div>` +
		`<div><div></div></div>` +
		`<div><div><span>end</span></div></div></code></pre>`
	lines := unboxAll(Render(src, opts(80)))
	want := []string{"keys:", "  - a", "", "end"}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q, want %q", lines, want)
	}
}

func TestPre_BrBreaksLines(t *testing.T) {
	lines := unboxAll(Render(`<pre>one<br>two</pre>`, opts(80)))
	if strings.Join(lines, "\n") != "one\ntwo" {
		t.Errorf("got %q", lines)
	}
}
