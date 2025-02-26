package lib

import (
	"sort"
	"time"
)

// Post represents a single post in a feed
type Post struct {
	UUID    string `json:"uuid"`
	Title   string `json:"-"`
	Content string `json:"-"`
	Link    string `json:"-"`
	Date    string `json:"-"`
	ID      int    `json:"-"`
	Read    bool   `json:"read"`
}

// Feed represents a single feed
type Feed struct {
	Title string `json:"-"`
	URL   string `json:"URL"`
	Posts []Post `json:"posts"`
	ID    int    `json:"-"`
}

// Feeds represents a collection of feeds
type Feeds []Feed

func (f Feeds) sort(urls []string) Feeds {
	// Create a map to store the index of each string in the url array
	urlMap := make(map[string]int)
	for i, str := range urls {
		urlMap[str] = i
	}

	// Sort the second set of strings based on the index in the first array
	sort.SliceStable(f, func(i, j int) bool {
		return urlMap[f[i].URL] < urlMap[f[j].URL]
	})

	return f
}

// GetTotalUnreads returns the total number of unread posts in a feed
func (f Feed) GetTotalUnreads() int {
	total := 0
	for _, post := range f.Posts {
		if !post.Read {
			total++
		}
	}
	return total
}

// GetTotalUnreads returns the total number of unread posts in all feeds
func (f Feeds) GetTotalUnreads() int {
	total := 0
	for _, feed := range f {
		total += feed.GetTotalUnreads()
	}
	return total
}

// silly leah thinks this is chatgpt-ed but NO. I wrote this myself. I'm just that good.
// also a bit of nix inspired me to write this `foldl recursiveUpdate { } importedLibs`
// okay maybe it was beacuse of the comments not actually the code, kinda fair.
func (feeds *Feeds) mergeFeeds(otherFeeds Feeds) {
	// Create a map to hold posts' read state from feeds1 by their UUID for quick lookup
	readStatusMap := make(map[string]bool)

	// Iterate through otherFeeds and map their posts by UUID
	for _, feed := range otherFeeds {
		for _, post := range feed.Posts {
			readStatusMap[post.UUID] = post.Read
		}
	}

	// Iterate through feeds and merge their posts into feeds1 based on UUID
	for i := range *feeds {
		for j := range (*feeds)[i].Posts {
			if readStatus, exists := readStatusMap[(*feeds)[i].Posts[j].UUID]; exists {
				(*feeds)[i].Posts[j].Read = readStatus
			}
		}
	}
}

// SortPostsByDate sorts an array of Post structs by the Date field.
func SortPosts(posts []Post) error {
	dateFormat := UserConfig.DateFormat

	sort.Slice(posts, func(i, j int) bool {
		// Parse the dates for the current comparison
		dateI, errI := time.Parse(dateFormat, posts[i].Date)
		dateJ, errJ := time.Parse(dateFormat, posts[j].Date)

		if errI != nil || errJ != nil {
			return false
		}

		// Compare the parsed dates
		return dateI.After(dateJ)
	})

	return nil
}
