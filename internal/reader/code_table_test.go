package reader

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

const goSrc = "func main() {}"

// unbox strips a code box line back to its text: styles, the two-cell
// padding and the fill to the width.
func unbox(line string) string {
	return strings.TrimRight(strings.TrimPrefix(ansi.Strip(line), "  "), " ")
}

// unboxAll is a lone code box's text lines, without its padding rows.
func unboxAll(doc Document) []string {
	if len(doc.Lines) < 2 {
		return nil
	}
	out := make([]string, 0, len(doc.Lines)-2)
	for _, l := range doc.Lines[1 : len(doc.Lines)-1] {
		out = append(out, unbox(l))
	}
	return out
}

func unboxed(doc Document) string { return strings.Join(unboxAll(doc), "\n") }

func TestCode_Highlighted(t *testing.T) {
	doc := Render(`<pre><code class="language-go">`+goSrc+`</code></pre>`, opts(80))
	if unboxed(doc) != goSrc {
		t.Errorf("stripped code changed: %q", unboxed(doc))
	}
	out := doc.String()
	plainCode := Render(`<pre><code class="language-text">`+goSrc+`</code></pre>`, opts(80)).String()
	if out == plainCode {
		t.Errorf("go code should be highlighted differently from plain text")
	}
}

func TestCode_UnknownLanguagePlain(t *testing.T) {
	if got := unboxed(Render(`<pre><code class="language-nope">`+goSrc+`</code></pre>`, opts(80))); got != goSrc {
		t.Errorf("got %q", got)
	}
}

func TestCode_UnknownStyleFallsBack(t *testing.T) {
	o := opts(80)
	o.Theme.CodeTheme = "does-not-exist"
	doc := Render(`<pre><code class="language-go">`+goSrc+`</code></pre>`, o)
	out := doc.String()
	if unboxed(doc) != goSrc || !strings.Contains(out, "\x1b[") {
		t.Errorf("expected highlighted output with fallback style, got %q", out)
	}
}

func TestCode_HighlightedLinesTruncate(t *testing.T) {
	src := "var x = \"" + strings.Repeat("a", 100) + "\""
	doc := Render(`<pre><code class="language-go">`+src+`</code></pre>`, opts(30))
	if len(doc.Lines) != 3 || ansi.StringWidth(doc.Lines[1]) != 30 || !strings.HasSuffix(unbox(doc.Lines[1]), "…") {
		t.Errorf("highlighted long line should truncate to 30 with …: %q", plain(doc))
	}
}

// Tables are drawn as glamour does: inner borders only, every cell padded a
// cell each side.
func TestTable_Grid(t *testing.T) {
	lines := plain(Render(`<table><tr><th>A</th><th>Bee</th></tr><tr><td>1</td><td>2</td></tr></table>`, opts(80)))
	want := []string{" A │ Bee ", "───┼─────", " 1 │ 2   "}
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

const nixSrc = `{ pkgs, lib, ... }:
{
  environment.systemPackages = with pkgs; [ git vim ];
  services.openssh.enable = true;
}`

// A code block that names no language is highlighted when its language can
// be told from the code, and left plain when it can't.
func TestCode_DetectsUntaggedLanguage(t *testing.T) {
	for _, tc := range []struct{ code, want string }{
		{nixSrc, "Nix"},
		{"#!/bin/sh\necho hi", "Bash"},
		{"hello world, this is just some output", "fallback"},
		{"", "fallback"},
	} {
		if got := lexer("", "", tc.code).Config().Name; got != tc.want {
			t.Errorf("lexer for %q = %s, want %s", tc.code, got, tc.want)
		}
	}
	if got := lexer("text", "", nixSrc).Config().Name; got != "plaintext" {
		t.Errorf("a named language wins over detection, got %s", got)
	}
	tagged := Render(`<pre><code class="language-nix">`+nixSrc+`</code></pre>`, opts(80)).String()
	if untagged := Render(`<pre><code>`+nixSrc+`</code></pre>`, opts(80)).String(); untagged != tagged {
		t.Errorf("untagged nix should highlight as tagged nix")
	}
}

func TestLooksLikeNix(t *testing.T) {
	for code, want := range map[string]bool{
		nixSrc: true,
		"{\n  sops = {\n    defaultSopsFile = ./secrets/shush.yaml;\n  };\n}":           true,
		"{\n  services.foo = {\n    enable = true;\n  };\n}":                            true,
		"{ pkgs ? import <nixpkgs> {} }:\npkgs.mkShell {\n  packages = [ pkgs.go ];\n}": true,
		"module.exports = {\n  name: 'x',\n};\nconst y = 1;\nreturn y;":                 false,
		"fn main() {\n    let x = 1;\n    println!(\"{x}\");\n}":                        false,
		"example.org {\n\treverse_proxy localhost:8080\n}":                              false,
		"DATA=/var/lib/x\nmkdir -p \"$DATA\"":                                           false,
		"keys:\n  - &isabel age1qyqs\ncreation_rules:\n  - path_regex: x":               false,
	} {
		if got := looksLikeNix(code); got != want {
			t.Errorf("looksLikeNix(%q) = %v, want %v", code, got, want)
		}
	}
}
