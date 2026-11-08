package reader

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// langHints is everything a code block says about its own language, apart
// from the code.
type langHints struct {
	Classes  []string // class attributes of the <pre> and its <code>
	Data     []string // data-language / data-lang attributes of the same
	Filename string   // a figcaption naming the file the snippet comes from
}

// language is the name a block is tagged with: "language-x" or "lang-x" in a
// class, else a data-language / data-lang attribute. Empty when untagged.
func (h langHints) language() string {
	for _, classes := range h.Classes {
		for class := range strings.FieldsSeq(classes) {
			for _, p := range []string{"language-", "lang-"} {
				if lang, ok := strings.CutPrefix(class, p); ok {
					return lang
				}
			}
		}
	}
	for _, d := range h.Data {
		if d = strings.TrimSpace(d); d != "" {
			return d
		}
	}
	return ""
}

// resolveLanguage picks the lexer for a code block: the tagged language, else
// the one the file name says, else what the code looks like. A block that
// names a language or a file is never second-guessed by the code. nil means
// no lexer applies: draw it plain.
func resolveLanguage(h langHints, code string) chroma.Lexer {
	lang := h.language()
	if lang != "" {
		if l := lexers.Get(lang); l != nil {
			return l
		}
	}
	if h.Filename != "" && !strings.ContainsAny(h.Filename, " /") {
		if l := lexers.Match(h.Filename); l != nil {
			return l
		}
	}
	if lang == "" && h.Filename == "" {
		return detect(code)
	}
	return nil
}

// codeLexer is resolveLanguage, with plain text for none so every block is
// drawn in the code theme's own colours.
func codeLexer(h langHints, code string) chroma.Lexer {
	l := resolveLanguage(h, code)
	if l == nil {
		l = lexers.Fallback
	}
	return chroma.Coalesce(l)
}

// detect guesses code's language from chroma's analysers, trusting only a
// confident score: their weak guesses misfire (GDScript for Go or Caddyfiles).
// It returns nil when unsure.
func detect(code string) chroma.Lexer {
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
