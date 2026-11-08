package reader

import (
	"image/color"
	"math"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/net/html"
)

// codeLanguage reads "language-x" / "lang-x" from <pre> or its <code> child.
func codeLanguage(pre *html.Node) string {
	nodes := []*html.Node{pre}
	for c := pre.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "code" {
			nodes = append(nodes, c)
		}
	}
	for _, n := range nodes {
		for class := range strings.FieldsSeq(attr(n, "class")) {
			for _, p := range []string{"language-", "lang-"} {
				if lang, ok := strings.CutPrefix(class, p); ok {
					return lang
				}
			}
		}
	}
	return ""
}

// codeTitle is the text of a <figcaption> directly before pre, which blogs
// use to name the file a snippet comes from.
func codeTitle(pre *html.Node) string {
	if c := prevElement(pre); c != nil && c.Data == "figcaption" {
		return strings.Join(strings.Fields(textContent(c)), " ")
	}
	return ""
}

func prevElement(n *html.Node) *html.Node {
	return siblingElement(n.PrevSibling, func(c *html.Node) *html.Node { return c.PrevSibling })
}

func nextElement(n *html.Node) *html.Node {
	return siblingElement(n.NextSibling, func(c *html.Node) *html.Node { return c.NextSibling })
}

// siblingElement walks from c with step to the first element, skipping
// whitespace; it returns nil if non-blank text comes first.
func siblingElement(c *html.Node, step func(*html.Node) *html.Node) *html.Node {
	for ; c != nil; c = step(c) {
		if c.Type == html.ElementNode {
			return c
		}
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) != "" {
			return nil
		}
	}
	return nil
}

// lexer picks chroma's lexer by language name, else by file name, else by
// what the code looks like, else plain text, so every block is drawn in the
// code theme's own colours.
func lexer(lang, filename, code string) chroma.Lexer {
	var l chroma.Lexer
	if lang != "" {
		l = lexers.Get(lang)
	}
	if l == nil && filename != "" && !strings.ContainsAny(filename, " /") {
		l = lexers.Match(filename)
	}
	if l == nil && lang == "" && filename == "" {
		l = detect(code)
	}
	if l == nil {
		l = lexers.Fallback
	}
	return chroma.Coalesce(l)
}

// detect guesses code's language, or returns nil when unsure. Nix, common in
// feeds but unknown to chroma's analysers, is checked first; then chroma's
// analysers, trusting only a confident score: their weak guesses misfire
// (GDScript for Go or Caddyfiles, MySQL for Nix).
func detect(code string) chroma.Lexer {
	if looksLikeNix(code) {
		return lexers.Get("nix")
	}
	var best chroma.Lexer
	var score float32
	for _, l := range lexers.GlobalLexerRegistry.Lexers {
		if a, ok := l.(chroma.Analyser); ok {
			if s := a.AnalyseText(code); s > score {
				best, score = l, s
			}
		}
	}
	if score < confident || best == nil || isPlainLexer(best) {
		return nil
	}
	return best
}

// confident is the analyser score detect trusts: shebangs, XML and PHP
// openers and the like score 1.
const confident = 0.8

func isPlainLexer(l chroma.Lexer) bool {
	name := l.Config().Name
	return l == lexers.Fallback || name == "plaintext" || name == "fallback"
}

// nixTells score how Nix-like code is: strong tells are constructs rarely
// seen elsewhere, weak ones are common in Nix but not unique to it.
var nixTells = []struct {
	re     *regexp.Regexp
	weight int
}{
	{regexp.MustCompile(`(?m)^\s*(?:\{.*)?\}\s*:\s*(?:\{|\w.*)?$`), 2}, // a function's argument set: "{ pkgs, ... }:"
	{regexp.MustCompile(`\bwith (?:pkgs|lib);`), 2},
	{regexp.MustCompile(`\b(?:mkDerivation|mkShell|mkOption|mkEnableOption|mkIf|mkDefault|mkForce|mkMerge|callPackage|buildGoModule|buildRustPackage|fetchFromGitHub|fetchTarball|mapAttrs|genAttrs|optionalString|concatStringsSep|builtins\.\w+)\b`), 2},
	{regexp.MustCompile(`\binherit\b`), 2},
	{regexp.MustCompile(`\$\{(?:pkgs|config|lib)\b`), 2},
	{regexp.MustCompile(`\bnixpkgs\.(?:url|lib)\b`), 2},
	{regexp.MustCompile(`(?m)^\s*'';\s*$`), 2},                                         // the end of an indented string
	{regexp.MustCompile(`(?m)^\s*[A-Za-z_][\w'-]*(?:\.[\w"'-]+)*\s*=\s*\S.*;\s*$`), 1}, // a binding: "a.b = 1;"
	{regexp.MustCompile(`(?m)^\s*\};\s*$`), 1},
	{regexp.MustCompile(`(?m)=\s*\{\s*$`), 1},
	{regexp.MustCompile(`=\s*\.{1,2}/\S*;`), 1}, // a path
	{regexp.MustCompile(`\b(?:pkgs|lib|config|inputs)\.[A-Za-z]`), 1},
	{regexp.MustCompile(`\b(?:return|var\s+\w|const\s+\w|function\s*\()|=>`), -2}, // JavaScript and kin
}

// looksLikeNix reports whether code scores as Nix.
func looksLikeNix(code string) bool {
	score := 0
	for _, t := range nixTells {
		if t.re.MatchString(code) {
			score += t.weight
		}
	}
	return score >= 3
}

// highlight returns code as styled lines using chroma's lexer, style and
// terminal formatter, or nil if chroma fails.
func highlight(code string, l chroma.Lexer, styleName string) []string {
	it, err := l.Tokenise(nil, code)
	if err != nil {
		return nil
	}
	var b strings.Builder
	// styles.Get returns styles.Fallback for unknown names.
	if err := formatters.TTY16m.Format(&b, styles.Get(styleName), it); err != nil {
		return nil
	}
	// Tokens such as block comments span lines; Wrap re-opens their style on
	// each line so every line stands alone. The width never forces a break.
	out := lipgloss.Wrap(strings.TrimRight(b.String(), "\n"), math.MaxInt32, "")
	return strings.Split(out, "\n")
}

// codeBackground is the code theme's background colour, or nil if it has none.
func codeBackground(styleName string) color.Color {
	bg := styles.Get(styleName).Get(chroma.Background).Background
	if !bg.IsSet() {
		return nil
	}
	return lipgloss.Color(bg.String())
}

// codeBox draws lines on bg with a row of padding above and below and two
// cells each side (less when narrow), every line exactly width cells wide.
// Lines too long are truncated, never wrapped.
func codeBox(lines []string, width int, bg color.Color) []string {
	open := style{bg: bg}.open()
	pad := 2
	switch {
	case width < 3:
		pad = 0
	case width < 12:
		pad = 1
	}
	inner := width - 2*pad
	side := strings.Repeat(" ", pad)
	blank := open + strings.Repeat(" ", width) + ansi.ResetStyle
	out := make([]string, 0, len(lines)+2)
	out = append(out, blank)
	for _, line := range lines {
		line = ansi.Truncate(line, inner, "…")
		gap := strings.Repeat(" ", max(inner-ansi.StringWidth(line), 0))
		if open != "" {
			// Highlighted tokens end in resets, which would also clear the
			// background: re-open it after each one.
			line = strings.ReplaceAll(strings.ReplaceAll(line, "\x1b[0m", ansi.ResetStyle), ansi.ResetStyle, ansi.ResetStyle+open)
		}
		out = append(out, open+side+line+gap+side+ansi.ResetStyle)
	}
	return append(out, blank)
}
