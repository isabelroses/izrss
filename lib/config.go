// Package lib common library functions
package lib

import (
	"log"
	"os"
	"strings"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

func getConfigFile(file string) string {
	configFile, err := xdg.ConfigFile("izrss/" + file)
	if err != nil {
		log.Fatalf("could not find config file: %v", err)
	}
	return configFile
}

// ParseUrls reads the URLs from the config file and returns them as a slice
func ParseUrls() []string {
	urlsFile := getConfigFile("urls")

	urlsRaw, err := os.ReadFile(urlsFile)
	if err != nil {
		log.Fatalf("could not read file: %v", err)
		return nil
	}

	// Convert byte slice to string
	rawString := string(urlsRaw)

	// Split string into individual URLs based on newline character
	urls := strings.Split(rawString, "\n")

	filteredUrls := []string{}
	for _, url := range urls {
		trimmedURL := strings.TrimSpace(url)
		if trimmedURL != "" {
			filteredUrls = append(filteredUrls, trimmedURL)
		}
	}

	return filteredUrls
}

// LoadConfig loads the users configuration file and applies it to the config struct
func LoadConfig() {
	configFile := getConfigFile("config.toml")
	configRaw, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("could not read file: %v", err)
	}

	if err := toml.Unmarshal(configRaw, &UserConfig); err != nil {
		log.Fatalf("could not unmarshal config: %v", err)
	}
}

// UserConfig is the global user configuration
var UserConfig = config{
	DateFormat: "02/01/2006",
	Colors: colors{
		Text:       "#cdd6f4",
		Inverttext: "#1e1e2e",
		Subtext:    "#a6adc8",
		Accent:     "#74c7ec",
		Borders:    "#313244",
	},
}

// Config is the struct that holds the configuration
type config struct {
	DateFormat string `toml:"dateformat"`
	Colors     colors `toml:"colors"`
}

type colors struct {
	Text       string `toml:"text"`
	Inverttext string `toml:"inverttext"`
	Subtext    string `toml:"subtext"`
	Accent     string `toml:"accent"`
	Borders    string `toml:"borders"`
}
