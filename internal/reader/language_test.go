package reader

import "testing"

// name is the resolved lexer's name, or "none".
func name(h langHints, code string) string {
	if l := resolveLanguage(h, code); l != nil {
		return l.Config().Name
	}
	return "none"
}

func TestResolveLanguage(t *testing.T) {
	const goCode = "func main() {}"
	nixCode := "{ pkgs, lib, ... }:\n{\n  environment.systemPackages = with pkgs; [ git vim ];\n  services.openssh.enable = true;\n}"
	cls := func(c ...string) langHints { return langHints{Classes: c} }
	tests := []struct {
		name string
		h    langHints
		code string
		want string
	}{
		{"language- class", cls("language-go"), goCode, "Go"},
		{"lang- class", cls("lang-python"), "x = 1", "Python"},
		{"among other classes", cls("highlight language-rust sourceCode"), "fn main() {}", "Rust"},
		{"on the code child as well as the pre", cls("", "language-go"), goCode, "Go"},
		{"data-language", langHints{Data: []string{"", "go"}}, goCode, "Go"},
		{"tagged language wins over what the code looks like", cls("language-text"), "#!/bin/sh\necho hi", "plaintext"},
		{"unknown tagged language is plain, not guessed", cls("language-nope"), "#!/bin/sh\necho hi", "none"},
		{"filename", langHints{Filename: "main.go"}, "package main", "Go"},
		{"filename with an extension chroma knows", langHints{Filename: "flake.nix"}, "{ }", "Nix"},
		{"tag beats filename", langHints{Classes: []string{"language-python"}, Filename: "main.go"}, "x = 1", "Python"},
		{"unknown tag falls back to the filename", langHints{Classes: []string{"language-nope"}, Filename: "main.go"}, "package main", "Go"},
		{"a caption that is not a file name does not detect either", langHints{Filename: "My notes about this"}, "#!/bin/sh\necho hi", "none"},
		{"untagged shebang is detected", langHints{}, "#!/bin/sh\necho hi", "Bash"},
		{"untagged weak guess is left plain", langHints{}, goCode, "none"},
		{"untagged prose is plain", langHints{}, "hello world, this is just some output", "none"},
		{"empty is plain", langHints{}, "", "none"},
		{"untagged nix is no longer special-cased", langHints{}, nixCode, "none"},
		{"tagged nix still highlights", cls("language-nix"), nixCode, "Nix"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := name(tt.h, tt.code); got != tt.want {
				t.Errorf("resolveLanguage = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCodeLexer_NoneIsPlain(t *testing.T) {
	if got := codeLexer(langHints{}, "just output").Config().Name; got != "fallback" {
		t.Errorf("got %s, want the plain fallback", got)
	}
}
