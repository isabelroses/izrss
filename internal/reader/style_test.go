package reader

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// H2–H6 are drawn as glamour does: the "##" marker and the text bold in the
// heading colour.
func TestHeading_Marker(t *testing.T) {
	th := testTheme()
	out := Render(`<h3>Title</h3>`, opts(40)).Lines[0]
	if got := ansi.Strip(out); got != "### Title" {
		t.Fatalf("got %q", got)
	}
	want := style{bold: true, fg: th.Heading}.open() + "### Title"
	if !strings.HasPrefix(out, want) {
		t.Errorf("heading should be bold in the heading colour, marker included: %q", out)
	}
}

// H1 is a chip: the text, padded, on the heading colour.
func TestHeading_H1Chip(t *testing.T) {
	th := testTheme()
	lines := Render(`<h1>Big title</h1>`, opts(40)).Lines
	if got := ansi.Strip(lines[0]); got != "\u00a0Big title\u00a0" {
		t.Fatalf("got %q", got)
	}
	chip := style{bold: true, fg: th.Chip, bg: th.Heading}.open()
	if !strings.HasPrefix(lines[0], chip) {
		t.Errorf("h1 should sit on the heading colour: %q", lines[0])
	}
}

// Links are drawn as glamour does: the text underlined in the link colour,
// then the URL, dimmed; both carry the OSC 8 target.
func TestLinks_TextThenURL(t *testing.T) {
	th := testTheme()
	doc := Render(`<p>go <a href="https://x.dev/a">read</a> now</p>`, opts(80))
	if got := plain(doc)[0]; got != "go read https://x.dev/a now" {
		t.Errorf("got %q", got)
	}
	out := doc.String()
	text := style{fg: th.Link, underline: true, link: "https://x.dev/a"}.open() + "read"
	if !strings.Contains(out, text) {
		t.Errorf("want link text styled %q in %q", text, out)
	}
	url := style{fg: th.Subtle, link: "https://x.dev/a"}.open() + "https://x.dev/a"
	if !strings.Contains(out, url) {
		t.Errorf("want the URL dimmed %q in %q", url, out)
	}
}

// The URL isn't repeated when the text already is the URL, and in-page
// anchors (footnotes) show only their text.
func TestLinks_URLShownOnce(t *testing.T) {
	for src, want := range map[string]string{
		`<p><a href="https://x.dev/a">https://x.dev/a</a></p>`: "https://x.dev/a",
		`<p><a href="https://x.dev/a/">x.dev/a</a></p>`:        "x.dev/a",
		`<p>see<a href="#fn1">1</a></p>`:                       "see1",
		`<p><a href="javascript:alert(1)">bad</a></p>`:         "bad",
	} {
		if got := plain(Render(src, opts(80)))[0]; got != want {
			t.Errorf("%s: got %q, want %q", src, got, want)
		}
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

// Code blocks are a box on the code theme's background, a row of padding
// above and below and two cells each side, exactly the text width.
func TestPre_Box(t *testing.T) {
	doc := Render("<pre><code class=\"language-go\">package main\n\nfunc main() {}</code></pre>", opts(30))
	if len(doc.Lines) != 5 {
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
	lines := plain(doc)
	if blank := strings.Repeat(" ", 30); lines[0] != blank || lines[4] != blank {
		t.Errorf("code should have a padding row above and below: %q", lines)
	}
	if got := lines[1]; got != "  package main"+strings.Repeat(" ", 16) {
		t.Errorf("code should be padded by two cells: %q", got)
	}
}

// A code block keeps a blank line from the text around it.
func TestPre_Spaced(t *testing.T) {
	lines := plain(Render("<p>before</p><pre>x</pre><p>after</p>", opts(20)))
	if len(lines) != 7 || lines[1] != "" || lines[5] != "" {
		t.Errorf("want text, blank, 3 box rows, blank, text: %q", lines)
	}
}

// A <figcaption> naming a file titles the code block and picks its language.
func TestPre_FigcaptionFilename(t *testing.T) {
	src := `<figure><figcaption><span>main.go</span></figcaption><pre><code>package main</code></pre></figure>`
	doc := Render(src, opts(40))
	lines := unboxAll(doc)
	if len(lines) != 2 || lines[0] != "main.go" || lines[1] != "package main" {
		t.Fatalf("want title row then code, got %q", lines)
	}
	plainGo := Render(`<pre><code>package main</code></pre>`, opts(40)).Lines[1]
	if doc.Lines[2] == plainGo {
		t.Error("the filename should select a highlighter")
	}
}

func TestSummary_Marker(t *testing.T) {
	lines := plain(Render(`<details><summary>More</summary><p>body</p></details>`, opts(40)))
	if lines[0] != "▸ More" {
		t.Errorf("summary should be marked: %q", lines)
	}
}

// Options.Margin indents the whole document like glamour's document margin:
// text wraps inside it, blank lines stay empty and image lines still point
// at their rows.
func TestMargin_IndentsDocument(t *testing.T) {
	o := opts(20)
	o.Margin = 2
	o.ShowImages = true
	o.Ready = map[string][]string{"https://example.com/a.png": {"A1", "A2"}}
	doc := Render(`<p>one two three four five six</p><img src="https://example.com/a.png"><p>end</p>`, o)
	want := []string{"  one two three", "  four five six", "", "  A1", "  A2", "", "  end"}
	if got := plain(doc); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q, want %q", got, want)
	}
	if len(doc.Images) != 1 || doc.Images[0].Line != 3 {
		t.Errorf("image line should still point at its first row: %+v", doc.Images)
	}
	if ContentWidth(o) != 16 {
		t.Errorf("ContentWidth = %d, want 16", ContentWidth(o))
	}
}
