package lib

import "sort"

type Post struct {
	Title   string
	Content string
	Link    string
	Date    string
	ID      int
}

type Feed struct {
	Title string
	URL   string
	Posts []Post
	ID    int
}

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
