package lib

import "sort"

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

func mergeFeeds(feeds1, feeds2 Feeds) Feeds {
	// Create a map to hold posts from feeds1 by their UUID for quick lookup
	postMap := make(map[string]*Post)

	// Iterate through feeds1 and map their posts by UUID
	for i := range feeds1 {
		for j := range feeds1[i].Posts {
			postMap[feeds1[i].Posts[j].UUID] = &feeds1[i].Posts[j]
		}
	}

	// Iterate through feeds2 and merge their posts into feeds1 based on UUID
	for i := range feeds2 {
		for j := range feeds2[i].Posts {
			if post1, exists := postMap[feeds2[i].Posts[j].UUID]; exists {
				// Update the existing post in feeds1 with the one from feeds2
				post1.Read = feeds2[i].Posts[j].Read
			}
		}
	}

	return feeds1
}
