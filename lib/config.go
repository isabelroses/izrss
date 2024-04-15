package lib

import (
	"log"
	"os"
	"strings"

	"github.com/adrg/xdg"
)

func ParseUrls() []string {
	urlsFile, err := xdg.ConfigFile("izrss/urls")
	if err != nil {
		log.Fatalf("could not find config file: %v", err)
		return nil
	}

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
