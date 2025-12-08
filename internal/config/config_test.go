package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}

	if cfg.Home != "home" {
		t.Errorf("Expected Home 'home', got %q", cfg.Home)
	}

	if cfg.DateFormat != "02/01/2006" {
		t.Errorf("Expected DateFormat '02/01/2006', got %q", cfg.DateFormat)
	}

	if len(cfg.Urls) != 0 {
		t.Errorf("Expected empty Urls, got %d items", len(cfg.Urls))
	}

	if cfg.Reader.ReadThreshold != 0.8 {
		t.Errorf("Expected ReadThreshold 0.8, got %f", cfg.Reader.ReadThreshold)
	}

	if cfg.Colors.Accent != "#74c7ec" {
		t.Errorf("Expected Accent '#74c7ec', got %q", cfg.Colors.Accent)
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/to/config.toml")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return defaults when file doesn't exist
	if cfg.Home != "home" {
		t.Errorf("Expected default Home, got %q", cfg.Home)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "izrss-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configPath := filepath.Join(tmpDir, "config.toml")
	configContent := `
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
`

	err = os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Home != "custom-home" {
		t.Errorf("Expected Home 'custom-home', got %q", cfg.Home)
	}

	if cfg.DateFormat != "2006-01-02" {
		t.Errorf("Expected DateFormat '2006-01-02', got %q", cfg.DateFormat)
	}

	if len(cfg.Urls) != 2 {
		t.Errorf("Expected 2 URLs, got %d", len(cfg.Urls))
	}

	if cfg.Urls[0] != "http://example.com/feed" {
		t.Errorf("Expected first URL 'http://example.com/feed', got %q", cfg.Urls[0])
	}

	if cfg.Reader.Theme != "dark" {
		t.Errorf("Expected Reader.Theme 'dark', got %q", cfg.Reader.Theme)
	}

	if cfg.Reader.ReadThreshold != 0.9 {
		t.Errorf("Expected Reader.ReadThreshold 0.9, got %f", cfg.Reader.ReadThreshold)
	}

	if cfg.Colors.Accent != "#ff0000" {
		t.Errorf("Expected Colors.Accent '#ff0000', got %q", cfg.Colors.Accent)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "izrss-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configPath := filepath.Join(tmpDir, "config.toml")
	configContent := `
urls = ["http://example.com/feed"]
dateformat = "Jan 2, 2006"
`

	err = os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Specified values should be loaded
	if len(cfg.Urls) != 1 {
		t.Errorf("Expected 1 URL, got %d", len(cfg.Urls))
	}

	if cfg.DateFormat != "Jan 2, 2006" {
		t.Errorf("Expected DateFormat 'Jan 2, 2006', got %q", cfg.DateFormat)
	}

	// Default values should be preserved
	if cfg.Home != "home" {
		t.Errorf("Expected default Home 'home', got %q", cfg.Home)
	}

	if cfg.Colors.Accent != "#74c7ec" {
		t.Errorf("Expected default Accent '#74c7ec', got %q", cfg.Colors.Accent)
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "izrss-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configPath := filepath.Join(tmpDir, "config.toml")
	invalidContent := `
this is not valid toml [[[
= missing key name
`

	err = os.WriteFile(configPath, []byte(invalidContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid TOML")
	}
}

func TestLoad_EmptyConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "izrss-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configPath := filepath.Join(tmpDir, "config.toml")
	err = os.WriteFile(configPath, []byte(""), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should still have defaults
	if cfg.Home != "home" {
		t.Errorf("Expected default Home, got %q", cfg.Home)
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	// This will try to load from default XDG path
	// We can't easily test the XDG path, but we can verify it doesn't crash
	cfg, err := Load("")
	if err != nil {
		// It's okay if the default path doesn't exist
		t.Logf("Load with empty path returned error (expected if no XDG config): %v", err)
	}
	if cfg != nil && cfg.Home != "home" {
		t.Errorf("Expected default Home if config loaded, got %q", cfg.Home)
	}
}

func TestColorsDefaults(t *testing.T) {
	cfg := Default()

	expectedColors := map[string]string{
		"Text":       "#cdd6f4",
		"Inverttext": "#1e1e2e",
		"Subtext":    "#a6adc8",
		"Accent":     "#74c7ec",
		"Borders":    "#313244",
	}

	if cfg.Colors.Text != expectedColors["Text"] {
		t.Errorf("Expected Text %q, got %q", expectedColors["Text"], cfg.Colors.Text)
	}
	if cfg.Colors.Inverttext != expectedColors["Inverttext"] {
		t.Errorf("Expected Inverttext %q, got %q", expectedColors["Inverttext"], cfg.Colors.Inverttext)
	}
	if cfg.Colors.Subtext != expectedColors["Subtext"] {
		t.Errorf("Expected Subtext %q, got %q", expectedColors["Subtext"], cfg.Colors.Subtext)
	}
	if cfg.Colors.Accent != expectedColors["Accent"] {
		t.Errorf("Expected Accent %q, got %q", expectedColors["Accent"], cfg.Colors.Accent)
	}
	if cfg.Colors.Borders != expectedColors["Borders"] {
		t.Errorf("Expected Borders %q, got %q", expectedColors["Borders"], cfg.Colors.Borders)
	}
}

func TestReaderDefaults(t *testing.T) {
	cfg := Default()

	if cfg.Reader.Size != "recomended" {
		t.Errorf("Expected Size 'recomended', got %v", cfg.Reader.Size)
	}

	if cfg.Reader.ReadThreshold != 0.8 {
		t.Errorf("Expected ReadThreshold 0.8, got %f", cfg.Reader.ReadThreshold)
	}

	if cfg.Reader.Theme != "" {
		t.Errorf("Expected empty Theme, got %q", cfg.Reader.Theme)
	}
}
