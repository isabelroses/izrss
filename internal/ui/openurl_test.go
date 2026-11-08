package ui

import "testing"

func TestBrowserURL(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"https://x.dev/a", "https://x.dev/a", true},
		{"http://x.dev/a?b=c", "http://x.dev/a?b=c", true},
		{"  https://x.dev/a  ", "https://x.dev/a", true},
		{"https://x.dev/a&calc", "https://x.dev/a&calc", true}, // passed as one argv entry, never through a shell
		{"javascript:alert(1)", "", false},
		{"file:///etc/passwd", "", false},
		{"http://", "", false},
		{"ftp://x.dev/a", "", false},
		{"", "", false},
		{"x.dev/a", "", false},
		{"https://x.dev/a\n&calc", "", false},
	}
	for _, tt := range tests {
		got, err := browserURL(tt.in)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("browserURL(%q) = %q, %v; want %q, ok=%v", tt.in, got, err, tt.want, tt.ok)
		}
	}
}

func TestBrowserCommand(t *testing.T) {
	const u = "https://x.dev/a&calc"
	tests := []struct {
		goos string
		wsl  bool
		want []string
	}{
		{"darwin", false, []string{"open", u}},
		{"linux", false, []string{"xdg-open", u}},
		{"windows", false, []string{"rundll32", "url.dll,FileProtocolHandler", u}},
	}
	for _, tt := range tests {
		got := browserCommand(tt.goos, tt.wsl, func(string) bool { return false }, u)
		if len(got) != len(tt.want) {
			t.Fatalf("%s: got %q, want %q", tt.goos, got, tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: got %q, want %q", tt.goos, got, tt.want)
			}
		}
	}

	has := func(bin string) bool { return bin == "wslview" }
	if got := browserCommand("linux", true, has, u); got[0] != "wslview" || got[1] != u {
		t.Errorf("wsl with wslview: %q", got)
	}
	if got := browserCommand("linux", true, func(string) bool { return false }, u); got[0] != "explorer.exe" || got[1] != u {
		t.Errorf("wsl without wslview: %q", got)
	}
}
