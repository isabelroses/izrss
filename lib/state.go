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
	return os.WriteFile(getStateFile(), json, 0644)
}

// ReadTracking reads the tracking state from a JSON file
func (feeds Feeds) ReadTracking() (Feeds, error) {
	fileStr := getStateFile()
	if _, err := os.Stat(fileStr); os.IsNotExist(err) {
		err := feeds.WriteTracking()
		if err != nil {
			log.Fatalf("could not write tracking file: %v", err)
		}
	}

	file, err := os.ReadFile(fileStr)
	if err != nil {
		return nil, err
	}

	feeds2 := Feeds{}
	err = json.Unmarshal(file, &feeds2)
	if err != nil {
		return nil, err
	}

	mergedFeed := mergeFeeds(feeds, feeds2)

	return mergedFeed, nil
}

func getStateFile() string {
	stateFile, err := xdg.StateFile("izrss/tracking.json")
	if err != nil {
		log.Fatalf("could not find state file: %v", err)
	}
	return stateFile
}
