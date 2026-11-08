package images

import "testing"

func TestEnabled(t *testing.T) {
	env := func(kv ...string) func(string) string {
		m := map[string]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return func(k string) string { return m[k] }
	}
	tests := []struct {
		name string
		mode string
		env  func(string) string
		want bool
	}{
		{"auto ghostty program", "auto", env("TERM_PROGRAM", "ghostty"), true},
		{"auto ghostty term", "auto", env("TERM", "xterm-ghostty"), true},
		{"auto kitty term", "auto", env("TERM", "xterm-kitty"), true},
		{"auto kitty window", "auto", env("KITTY_WINDOW_ID", "1"), true},
		{"auto other terminal", "auto", env("TERM", "xterm-256color"), false},
		{"auto in tmux", "auto", env("TERM_PROGRAM", "ghostty", "TMUX", "/tmp/tmux"), false},
		{"auto in screen", "auto", env("TERM", "xterm-kitty", "STY", "123.pts"), false},
		{"empty mode means auto", "", env("TERM", "xterm-kitty"), true},
		{"on forces", "on", env("TERM", "dumb", "TMUX", "x"), true},
		{"off disables", "off", env("TERM_PROGRAM", "ghostty"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Enabled(tt.mode, tt.env); got != tt.want {
				t.Errorf("Enabled = %v, want %v", got, tt.want)
			}
		})
	}
}
