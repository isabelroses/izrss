package config

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// captureLog is what the package logs until the test ends.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

// writeConfig writes content to a config.toml in a fresh temp dir.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	return path
}

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Home != "home" || cfg.DateFormat != "02/01/2006" || len(cfg.Urls) != 0 {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	r := cfg.Reader
	if r.Size != "recomended" || r.ReadThreshold != 0.8 || r.Theme != "" {
		t.Errorf("unexpected reader defaults: %+v", r)
	}
	want := Colors{Text: "#cdd6f4", Inverttext: "#1e1e2e", Subtext: "#a6adc8", Accent: "#74c7ec", Borders: "#313244"}
	if cfg.Colors != want {
		t.Errorf("Colors = %+v, want %+v", cfg.Colors, want)
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/to/config.toml")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Home != "home" {
		t.Errorf("Expected default Home, got %q", cfg.Home)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	cfg, err := Load(writeConfig(t, `
home = "custom-home"
dateformat = "2006-01-02"
urls = ["http://example.com/feed", "http://example.org/rss"]

[reader]
size = "large"
theme = "dark"
read_threshold = 0.9

[colors]
text = "#ffffff"
inverttext = "#000000"
subtext = "#cccccc"
accent = "#ff0000"
borders = "#333333"
`))
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if cfg.Home != "custom-home" || cfg.DateFormat != "2006-01-02" {
		t.Errorf("Home, DateFormat = %q, %q", cfg.Home, cfg.DateFormat)
	}
	if len(cfg.Urls) != 2 || cfg.Urls[0] != "http://example.com/feed" {
		t.Errorf("Urls = %q", cfg.Urls)
	}
	if cfg.Reader.Theme != "dark" || cfg.Reader.ReadThreshold != 0.9 {
		t.Errorf("Reader = %+v", cfg.Reader)
	}
	if cfg.Colors.Accent != "#ff0000" {
		t.Errorf("Expected Colors.Accent '#ff0000', got %q", cfg.Colors.Accent)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	cfg, err := Load(writeConfig(t, `
urls = ["http://example.com/feed"]
dateformat = "Jan 2, 2006"
`))
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if len(cfg.Urls) != 1 || cfg.DateFormat != "Jan 2, 2006" {
		t.Errorf("specified values not loaded: %+v", cfg)
	}
	if cfg.Home != "home" || cfg.Colors.Accent != "#74c7ec" {
		t.Errorf("defaults not preserved: %+v", cfg)
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	if _, err := Load(writeConfig(t, "\nthis is not valid toml [[[\n= missing key name\n")); err == nil {
		t.Error("Expected error for invalid TOML")
	}
}

func TestLoad_EmptyConfig(t *testing.T) {
	cfg, err := Load(writeConfig(t, ""))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Home != "home" {
		t.Errorf("Expected default Home, got %q", cfg.Home)
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	// This loads from the default XDG path, which may not exist; it must not
	// crash, and anything it does load starts from the defaults.
	cfg, err := Load("")
	if err != nil {
		t.Logf("Load with empty path returned error (expected if no XDG config): %v", err)
	}
	if cfg != nil && cfg.Home != "home" {
		t.Errorf("Expected default Home if config loaded, got %q", cfg.Home)
	}
}

func TestLoad_DedupesUrls(t *testing.T) {
	logged := captureLog(t)
	cfg, err := Load(writeConfig(t, `urls = ["https://a.example/feed", "https://b.example/feed", "https://a.example/feed", "https://c.example/feed", "https://b.example/feed"]`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://a.example/feed", "https://b.example/feed", "https://c.example/feed"}
	if !slices.Equal(cfg.Urls, want) {
		t.Errorf("Urls = %q, want %q (first occurrence kept, order preserved)", cfg.Urls, want)
	}
	for _, u := range []string{"duplicate feed url https://a.example/feed", "duplicate feed url https://b.example/feed"} {
		if !bytes.Contains(logged.Bytes(), []byte(u)) {
			t.Errorf("log %q lacks %q", logged, u)
		}
	}
}
