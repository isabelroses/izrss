package reader

import (
	"net/url"
	"strings"
	"unicode"
)

// clean removes control characters from untrusted feed text (except \t \n \r \f):
// references like &#27; decode to raw escapes that would reach the terminal.
func clean(s string) string {
	if !strings.ContainsFunc(s, isUnsafe) {
		return s
	}
	return strings.Map(func(r rune) rune {
		if isUnsafe(r) {
			return -1
		}
		return r
	}, s)
}

func isUnsafe(r rune) bool {
	switch r {
	case '\t', '\n', '\r', '\f':
		return false
	}
	return unicode.IsControl(r) // C0, DEL and C1
}

// safeLink reports whether u may be emitted as a terminal hyperlink.
func safeLink(u string) bool {
	p, err := url.Parse(u)
	if err != nil {
		return false
	}
	switch p.Scheme {
	case "http", "https":
		return p.Host != ""
	case "mailto":
		return p.Opaque != ""
	}
	return false
}
