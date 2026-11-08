package ui

import (
	"errors"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

// browserURL validates an untrusted feed link before it reaches the OS: only
// absolute http(s) URLs with a host are opened.
func browserURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", errors.New("invalid link")
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("not a web link")
	}
	return u.String(), nil
}

// browserCommand is the argv that opens u. No shell is involved, so the URL
// is always a single argument.
func browserCommand(goos string, wsl bool, has func(string) bool, u string) []string {
	switch {
	case goos == "windows":
		return []string{"rundll32", "url.dll,FileProtocolHandler", u}
	case goos == "darwin":
		return []string{"open", u}
	case wsl && has("wslview"):
		return []string{"wslview", u}
	case wsl:
		return []string{"explorer.exe", u}
	}
	return []string{"xdg-open", u}
}

// openURL opens a feed link in the default browser.
func openURL(raw string) error {
	u, err := browserURL(raw)
	if err != nil {
		return err
	}
	has := func(bin string) bool { _, err := exec.LookPath(bin); return err == nil }
	argv := browserCommand(runtime.GOOS, runtime.GOOS == "linux" && isWSL(), has, u)
	return exec.Command(argv[0], argv[1:]...).Start()
}

func isWSL() bool {
	out, err := exec.Command("uname", "-r").Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), "microsoft")
}
