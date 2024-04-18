package lib

import "sort"

type Post struct {
	UUID    string `json:"uuid"`
	Title   string `json:"-"`
	Content string `json:"-"`
	Link    string `json:"-"`
	Date    string `json:"-"`
	ID      int    `json:"-"`
	Read    bool   `json:"read"`
}

type Feed struct {
	Title string `json:"-"`
	URL   string `json:"URL"`
	Posts []Post `json:"posts"`
	ID    int    `json:"-"`
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

func (f Feed) GetTotalUnreads() int {
	total := 0
	for _, post := range f.Posts {
		if !post.Read {
			total++
		}
	}
	return total
}
