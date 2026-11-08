package reader

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Heading markers stay (they show the level) but are drawn quietly, so the
// heading text is what stands out.
func TestHeading_QuietMarker(t *testing.T) {
	th := testTheme()
	out := Render(`<h2>Title</h2>`, opts(40)).Lines[0]
	if got := ansi.Strip(out); got != "## Title" {
		t.Fatalf("got %q", got)
	}
	marker := style{fg: th.Subtle}.open() + "##"
	if !strings.HasPrefix(out, marker) {
		t.Errorf("marker should be subtle: %q", out)
	}
	if !strings.Contains(out, style{bold: true, fg: th.Heading}.open()+" Title") {
		t.Errorf("heading text should be bold in the heading colour: %q", out)
	}
}

// Links are shown by colour and an OSC 8 target, without underlines that
// clutter running text.
func TestLinks_ColourNoUnderline(t *testing.T) {
	out := Render(`<p><a href="https://x.dev/a">read</a></p>`, opts(80)).String()
	want := style{fg: testTheme().Link, link: "https://x.dev/a"}.open() + "read"
	if !strings.Contains(out, want) {
		t.Errorf("want link styled %q in %q", want, out)
	}
}

func TestInlineCode_Chip(t *testing.T) {
	doc := Render(`<p>run <code>go test</code> now</p>`, opts(80))
	if got := plain(doc)[0]; got != "run  go test  now" {
		t.Errorf("inline code should be padded with non-breaking spaces: %q", got)
	}
	if !strings.Contains(doc.Lines[0], style{fg: testTheme().Code, bg: codeBackground(testTheme().CodeTheme)}.open()) {
		t.Errorf("inline code should sit on the code background: %q", doc.Lines[0])
	}
}

// Code blocks are a box on the code theme's background, one cell of padding
// each side, exactly the text width.
func TestPre_Box(t *testing.T) {
	doc := Render("<pre><code class=\"language-go\">package main\n\nfunc main() {}</code></pre>", opts(30))
	if len(doc.Lines) != 3 {
		t.Fatalf("got %q", plain(doc))
	}
	bg := style{bg: codeBackground(testTheme().CodeTheme)}.open()
	for i, line := range doc.Lines {
		if w := ansi.StringWidth(line); w != 30 {
			t.Errorf("line %d is %d wide, want 30: %q", i, w, ansi.Strip(line))
		}
		if !strings.HasPrefix(line, bg) {
			t.Errorf("line %d should start on the background: %q", i, line)
		}
		// Every reset inside the line must re-open the background.
		body := strings.TrimSuffix(line, ansi.ResetStyle)
		for j := strings.Index(body, ansi.ResetStyle); j >= 0; j = strings.Index(body, ansi.ResetStyle) {
			body = body[j+len(ansi.ResetStyle):]
			if !strings.HasPrefix(body, bg) {
				t.Fatalf("line %d drops the background after a reset: %q", i, line)
			}
		}
	}
	if got := plain(doc)[0]; got != " package main"+strings.Repeat(" ", 17) {
		t.Errorf("code should be padded by one cell: %q", got)
	}
}

// A <figcaption> naming a file titles the code block and picks its language.
func TestPre_FigcaptionFilename(t *testing.T) {
	src := `<figure><figcaption><span>main.go</span></figcaption><pre><code>package main</code></pre></figure>`
	doc := Render(src, opts(40))
	lines := plain(doc)
	if len(lines) != 2 || strings.TrimSpace(lines[0]) != "main.go" || strings.TrimSpace(lines[1]) != "package main" {
		t.Fatalf("want title row then code, got %q", lines)
	}
	plainGo := Render(`<pre><code>package main</code></pre>`, opts(40)).Lines[0]
	if doc.Lines[1] == plainGo {
		t.Error("the filename should select a highlighter")
	}
}

func TestSummary_Marker(t *testing.T) {
	lines := plain(Render(`<details><summary>More</summary><p>body</p></details>`, opts(40)))
	if lines[0] != "▸ More" {
		t.Errorf("summary should be marked: %q", lines)
	}
}
