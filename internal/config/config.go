// Package config handles application configuration
package config

import (
	"fmt"
	"os"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

// Config holds the application configuration
type Config struct {
	Home       string   `toml:"home"`
	DateFormat string   `toml:"dateformat"`
	Urls       []string `toml:"urls"`
	Reader     Reader   `toml:"reader"`
	Colors     Colors   `toml:"colors"`
}

// Reader contains reader-specific configuration
type Reader struct {
	Size          any     `toml:"size"`
	Theme         string  `toml:"theme"`
	ReadThreshold float64 `toml:"read_threshold"`
}

// Colors contains UI color configuration
type Colors struct {
	Text       string `toml:"text"`
	Inverttext string `toml:"inverttext"`
	Subtext    string `toml:"subtext"`
	Accent     string `toml:"accent"`
	Borders    string `toml:"borders"`
}

// Default returns the default configuration
func Default() *Config {
	return &Config{
		Home:       "home",
		DateFormat: "02/01/2006",
		Urls:       []string{},
		Reader: Reader{
			Size:          "recomended",
			ReadThreshold: 0.8,
			Theme:         "",
		},
		Colors: Colors{
			Text:       "#cdd6f4",
			Inverttext: "#1e1e2e",
			Subtext:    "#a6adc8",
			Accent:     "#74c7ec",
			Borders:    "#313244",
		},
	}
}

// Load loads configuration from the specified path, or the default location if empty
func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		var err error
		path, err = configFile("config.toml")
		if err != nil {
			return nil, fmt.Errorf("getting config file path: %w", err)
		}
	}

	configRaw, err := os.ReadFile(path)
	if err != nil {
		// Config file doesn't exist, use defaults
		return cfg, nil
	}

	if err := toml.Unmarshal(configRaw, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return cfg, nil
}

func configFile(file string) (string, error) {
	configFile, err := xdg.ConfigFile("izrss/" + file)
	if err != nil {
		return "", fmt.Errorf("getting config file path: %w", err)
	}
	return configFile, nil
}
