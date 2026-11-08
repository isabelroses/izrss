package reader

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Feed HTML is untrusted: character references decode to raw control
// characters, which must never reach the terminal.
func TestSanitize_NoControlCharactersReachOutput(t *testing.T) {
	inputs := map[string]string{
		"text":      `<p>hi &#27;[2J&#27;]0;pwned&#7; there</p>`,
		"href":      `<p><a href="https://x.dev/&#27;]8;;evil&#7;">link</a></p>`,
		"alt":       `<img src="https://x.dev/a.png" alt="cat&#27;[31m">`,
		"code":      `<pre>x &#27;[2J y</pre>`,
		"table":     `<table><tr><td>a&#27;[2Jb</td></tr></table>`,
		"highlight": `<pre><code class="language-go">var s = "&#27;[2J"</code></pre>`,
	}
	for name, src := range inputs {
		t.Run(name, func(t *testing.T) {
			out := Render(src, opts(80)).String()
			// Strip the escapes we emit ourselves; anything left is injected.
			if visible := ansi.Strip(out); strings.ContainsFunc(visible, isControl) {
				t.Errorf("control characters leaked: %q", visible)
			}
			if strings.Contains(out, "\x1b]0;") || strings.Contains(out, "\x1b[2J") {
				t.Errorf("injected escape sequence present: %q", out)
			}
			// Every hyperlink target we emit must be free of control characters,
			// or it could terminate the OSC 8 sequence early.
			for _, seg := range strings.Split(out, "\x1b]8;")[1:] {
				target, _, _ := strings.Cut(seg, "\a")
				if strings.ContainsFunc(target, isControl) {
					t.Errorf("control character inside hyperlink target: %q", target)
				}
			}
		})
	}
}

func TestLinks_OnlySafeSchemes(t *testing.T) {
	for _, href := range []string{"javascript:alert(1)", "file:///etc/passwd", "data:text/html,x"} {
		out := Render(`<p><a href="`+href+`">x</a></p>`, opts(80)).String()
		if strings.Contains(out, "\x1b]8;;"+href[:4]) {
			t.Errorf("%s should not become a hyperlink: %q", href, out)
		}
	}
	if out := Render(`<p><a href="mailto:a@b.c">mail</a></p>`, opts(80)).String(); !strings.Contains(out, ansi.SetHyperlink("mailto:a@b.c")) {
		t.Errorf("mailto links should work: %q", out)
	}
}

func isControl(r rune) bool { return r < 0x20 && r != '\n' || r == 0x7f || (r >= 0x80 && r < 0xa0) }
