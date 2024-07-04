package lib

import (
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// OpenURL opens the specified URL in the default browser of the user.
// https://stackoverflow.com/questions/39320371/how-start-web-server-to-open-page-in-browser-in-golang
func OpenURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		// Check if running under WSL
		if isWSL() {
			// Use 'cmd.exe /c start' to open the URL in the default Windows browser
			cmd = "cmd.exe"
			args = []string{"/c", "start", url}
		} else {
			// Use xdg-open on native Linux environments
			cmd = "xdg-open"
			args = []string{url}
		}
	}

	return exec.Command(cmd, args...).Start()
}

// isWSL checks if the Go program is running inside Windows Subsystem for Linux
func isWSL() bool {
	releaseData, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(releaseData)), "microsoft")
}

// ConvertDate converts a date string to the user's preferred date format
func ConvertDate(dateString string) string {
	layoutList := []string{
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"Monday, 02-Jan-06 15:04:05 MST",
		"02 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 +0000",
		"02 Jan 2006 15:04:05 MST",
		"02-Jan-06 15:04:05 MST",
		"2006-02-01T15:04:05",
		"2006-01-02T15:04:05",
		"January 02, 2006",
		"02/Jan/2006",
		"02-Jan-2006",
		"2006-01-02",
		"01/02/2006",
		time.RFC3339,
	}

	var parsedDate time.Time
	var err error

	for _, layout := range layoutList {
		parsedDate, err = time.Parse(layout, dateString)
		if err == nil {
			break
		}
	}

	if err != nil {
		return dateString
	}

	date := parsedDate.Format(UserConfig.DateFormat)

	return date
}

// URLToDir converts a URL to a directory name
// https://isabelroses.com/feed.xml -> isabelroses_com_feed.xml
func URLToDir(url string) string {
	url = strings.ReplaceAll(url, "https://", "")
	url = strings.ReplaceAll(url, "http://", "")
	url = strings.ReplaceAll(url, "/", "_")
	// replace all dots but the last one
	dots := strings.Count(url, ".") - 1
	url = strings.Replace(url, ".", "_", dots)
	return url
}

func ReadSymbol(read bool) string {
	if read {
		return ""
	}
	return "•"
}
