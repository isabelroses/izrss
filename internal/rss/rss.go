// Package rss handles RSS feed fetching and parsing
package rss

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/isabelroses/izrss/internal/storage"
)

// Post represents a single post in a feed
type Post struct {
	UUID    string
	Title   string
	Content string
	Link    string
	Date    string
	ID      int
	Read    bool
}

// Feed represents a single feed
type Feed struct {
	Title string
	URL   string
	Posts []Post
	ID    int
}

// Feeds represents a collection of feeds
type Feeds []Feed

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

// SortPosts sorts posts by date in descending order
func SortPosts(posts []Post, dateFormat string) {
	sort.Slice(posts, func(i, j int) bool {
		dateI, errI := time.Parse(dateFormat, posts[i].Date)
		dateJ, errJ := time.Parse(dateFormat, posts[j].Date)
		if errI != nil || errJ != nil {
			return false
		}
		return dateI.After(dateJ)
	})
}

func (f Feeds) sort(urls []string) Feeds {
	urlMap := make(map[string]int)
	for i, str := range urls {
		urlMap[str] = i
	}

	sort.SliceStable(f, func(i, j int) bool {
		return urlMap[f[i].URL] < urlMap[f[j].URL]
	})

	return f
}

// ToggleRead toggles the read status of a post
func ToggleRead(feeds Feeds, feedID, postID int) {
	feeds[feedID].Posts[postID].Read = !feeds[feedID].Posts[postID].Read
}

// ReadAll marks all posts in a feed as read
func ReadAll(feeds Feeds, feedID int) {
	for i := range feeds[feedID].Posts {
		feeds[feedID].Posts[i].Read = true
	}
}

// MarkRead marks a post as read
func MarkRead(feeds Feeds, feedID, postID int) {
	feeds[feedID].Posts[postID].Read = true
}

// WriteTracking saves the tracking state to the database
func (feeds Feeds) WriteTracking(db *storage.DB) error {
	var statuses []storage.PostReadStatus
	for _, feed := range feeds {
		for _, post := range feed.Posts {
			statuses = append(statuses, storage.PostReadStatus{
				UUID:    post.UUID,
				FeedURL: feed.URL,
				Read:    post.Read,
			})
		}
	}
	return db.SavePostReadStatuses(statuses)
}

// ReadTracking reads the tracking state from the database
func (feeds *Feeds) ReadTracking(db *storage.DB) error {
	statuses, err := db.LoadPostReadStatuses()
	if err != nil {
		return err
	}

	for i := range *feeds {
		for j := range (*feeds)[i].Posts {
			if readStatus, exists := statuses[(*feeds)[i].Posts[j].UUID]; exists {
				(*feeds)[i].Posts[j].Read = readStatus
			}
		}
	}

	return nil
}

// Fetcher handles RSS feed fetching
type Fetcher struct {
	db         *storage.DB
	dateFormat string
}

// NewFetcher creates a new Fetcher
func NewFetcher(db *storage.DB, dateFormat string) *Fetcher {
	return &Fetcher{
		db:         db,
		dateFormat: dateFormat,
	}
}

// FetchURL fetches the content of a URL and returns it as a byte slice
func (f *Fetcher) FetchURL(url string, preferCache bool) ([]byte, error) {
	if preferCache {
		if data, err := f.db.LoadFeedCache(url); err == nil && data != nil {
			return data, nil
		}
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching URL %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if err := f.db.SaveFeedCache(url, body); err != nil {
		log.Printf("could not cache feed %s: %v", url, err)
	}

	return body, nil
}

// GetContentForURL fetches the content of a URL and returns it as a Feed
func (f *Fetcher) GetContentForURL(url string, preferCache bool) Feed {
	feed := f.setupReader(url, preferCache)

	if feed == nil {
		return Feed{
			Title: fmt.Sprintf("Error loading %s", url),
			URL:   url,
			Posts: []Post{},
		}
	}

	feedRet := Feed{
		Title: feed.Title,
		URL:   url,
		Posts: make([]Post, 0, len(feed.Items)),
	}

	for _, item := range feed.Items {
		feedRet.Posts = append(feedRet.Posts, f.createPost(item))
	}

	return feedRet
}

// GetPosts fetches the content of a URL and returns it as a slice of Posts
func (f *Fetcher) GetPosts(url string) []Post {
	feed := f.setupReader(url, false)
	if feed == nil {
		return []Post{}
	}

	posts := make([]Post, 0, len(feed.Items))
	for _, item := range feed.Items {
		posts = append(posts, f.createPost(item))
	}

	return posts
}

func (f *Fetcher) createPost(item *gofeed.Item) Post {
	content := item.Content
	if content == "" {
		content = item.Description
	}
	if content == "" {
		content = "This post does not contain any content.\nPress \"o\" to open the post in your preferred browser"
	}

	return Post{
		Title:   item.Title,
		Content: content,
		Link:    item.Link,
		Date:    convertDate(item.Published, f.dateFormat),
		UUID:    item.GUID,
	}
}

func (f *Fetcher) setupReader(url string, preferCache bool) *gofeed.Feed {
	data, err := f.FetchURL(url, preferCache)
	if err != nil {
		log.Printf("could not fetch feed %s: %v", url, err)
		return nil
	}

	if len(data) == 0 {
		return nil
	}

	fp := gofeed.NewParser()
	feed, err := fp.ParseString(string(data))
	if err != nil {
		log.Printf("could not parse feed %s: %v", url, err)
		return nil
	}

	return feed
}

// GetAllContent fetches the content of all URLs and returns it as Feeds
func (f *Fetcher) GetAllContent(urls []string, preferCache bool) Feeds {
	if !preferCache {
		if err := f.db.SetCacheTime(); err != nil {
			log.Printf("could not write cache time: %v", err)
		}
	}

	var wg sync.WaitGroup
	responses := make(chan Feed, len(urls))

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			responses <- f.GetContentForURL(u, preferCache)
		}(url)
	}

	go func() {
		wg.Wait()
		close(responses)
	}()

	feeds := make(Feeds, 0, len(urls))
	for response := range responses {
		feeds = append(feeds, response)
	}

	return feeds.sort(urls)
}

// CheckCache returns true if cached data should be used
func (f *Fetcher) CheckCache() bool {
	last, err := f.db.GetCacheTime()
	if err != nil {
		log.Printf("could not get cache time: %v", err)
		return false
	}

	if last == nil {
		return false
	}

	return time.Since(*last) <= 24*time.Hour
}

// Helper functions

func convertDate(dateString, dateFormat string) string {
	layouts := []string{
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"Monday, 02-Jan-06 15:04:05 MST",
		"02 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 +0000",
		"02 Jan 2006 15:04:05 MST",
		"02-Jan-06 15:04:05 MST",
		"2006-02-01T15:04:05",
		"2006-01-02T15:04:05",
		"January 02, 2006",
		"02/Jan/2006",
		"02-Jan-2006",
		"2006-01-02",
		"01/02/2006",
		time.RFC3339,
	}

	for _, layout := range layouts {
		if parsedDate, err := time.Parse(layout, dateString); err == nil {
			return parsedDate.Format(dateFormat)
		}
	}

	return dateString
}

// ReadSymbol returns a bullet character for unread posts, empty string for read
func ReadSymbol(read bool) string {
	if read {
		return ""
	}
	return "•"
}
