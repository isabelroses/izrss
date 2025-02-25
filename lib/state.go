package lib

import (
	"encoding/json"
	"log"
	"os"

	"github.com/adrg/xdg"
)

// ToggleRead toggles the read status of a post
func ToggleRead(feeds Feeds, feedID int, postID int) Feeds {
	postr := &feeds[feedID].Posts[postID]
	postr.Read = !postr.Read
	return feeds
}

// ReadAll marks all posts in a feed as read
func ReadAll(feeds Feeds, feedID int) Feeds {
	for i := range feeds[feedID].Posts {
		feeds[feedID].Posts[i].Read = true
	}
	return feeds
}

// MarkRead marks a post as read
func MarkRead(feeds Feeds, feedID int, postID int) Feeds {
	postr := &feeds[feedID].Posts[postID]
	postr.Read = true
	return feeds
}

// WriteTracking saves the tracking state to a JSON file
func (feeds Feeds) WriteTracking() error {
	json, err := json.Marshal(feeds)
	if err != nil {
		return err
	}
	return os.WriteFile(getStateFile("tracking.json"), json, 0644)
}

// ReadTracking reads the tracking state from a JSON file
func (feeds *Feeds) ReadTracking() error {
	fileStr := getStateFile("tracking.json")
	if _, err := os.Stat(fileStr); os.IsNotExist(err) {
		err := feeds.WriteTracking()
		if err != nil {
			log.Fatalf("could not write tracking file: %v", err)
		}
	}

	file, err := os.ReadFile(fileStr)
	if err != nil {
		return err
	}

	trackingData := Feeds{}
	err = json.Unmarshal(file, &trackingData)
	if err != nil {
		return err
	}

	feeds.mergeFeeds(trackingData)

	return nil
}

func getStateFile(file string) string {
	stateFile, err := xdg.StateFile("izrss/" + file)
	if err != nil {
		log.Fatalf("could not find state file: %v", err)
	}
	return stateFile
}
