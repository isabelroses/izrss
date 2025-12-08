// Package rss handles RSS feed fetching and parsing
package rss

import (
	"bytes"
	"encoding/gob"
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
	// Pre-calculate total size to avoid reallocation
	totalPosts := 0
	for _, feed := range feeds {
		totalPosts += len(feed.Posts)
	}

	statuses := make([]storage.PostReadStatus, 0, totalPosts)
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
	client     *http.Client
}

// NewFetcher creates a new Fetcher
func NewFetcher(db *storage.DB, dateFormat string) *Fetcher {
	// Configure HTTP client with connection pooling for concurrent requests
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}

	return &Fetcher{
		db:         db,
		dateFormat: dateFormat,
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

// FetchURL fetches the content of a URL and returns it as a byte slice
func (f *Fetcher) FetchURL(url string, preferCache bool) ([]byte, error) {
	if preferCache {
		if data, err := f.db.LoadFeedCache(url); err == nil && data != nil {
			return data, nil
		}
	}

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching URL %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	// Try parsed feed cache first (avoids XML parsing entirely)
	if preferCache {
		if cachedData, err := f.db.LoadParsedFeed(url); err == nil && cachedData != nil {
			var feed Feed
			decoder := gob.NewDecoder(bytes.NewReader(cachedData))
			if err := decoder.Decode(&feed); err == nil {
				return feed
			}
		}
	}

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

	// Cache the parsed feed for faster subsequent loads
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(&feedRet); err == nil {
		if err := f.db.SaveParsedFeed(url, buf.Bytes()); err != nil {
			log.Printf("failed to cache parsed feed %s: %v", url, err)
		}
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

	// When using cache, batch load all parsed feeds in one query
	if preferCache {
		return f.getAllContentFromCache(urls)
	}

	// Use worker pool for fresh fetches to limit concurrent HTTP requests
	const maxWorkers = 20
	semaphore := make(chan struct{}, maxWorkers)

	var wg sync.WaitGroup
	var mu sync.Mutex
	feeds := make(Feeds, 0, len(urls))

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			feed := f.GetContentForURL(u, false)

			mu.Lock()
			feeds = append(feeds, feed)
			mu.Unlock()
		}(url)
	}

	wg.Wait()

	return feeds.sort(urls)
}

// getAllContentFromCache loads all feeds from parsed cache in a single batch query
func (f *Fetcher) getAllContentFromCache(urls []string) Feeds {
	// Load all parsed feeds in one query instead of N queries
	cachedFeeds, err := f.db.LoadAllParsedFeeds()
	if err != nil {
		log.Printf("could not load parsed feeds cache: %v", err)
		cachedFeeds = make(map[string][]byte)
	}

	feeds := make(Feeds, 0, len(urls))
	for _, url := range urls {
		if cachedData, exists := cachedFeeds[url]; exists {
			var feed Feed
			decoder := gob.NewDecoder(bytes.NewReader(cachedData))
			if err := decoder.Decode(&feed); err == nil {
				feeds = append(feeds, feed)
				continue
			}
		}
		// Fallback to individual fetch if not in cache
		feeds = append(feeds, f.GetContentForURL(url, true))
	}
	return feeds
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
