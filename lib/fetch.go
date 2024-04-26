package lib

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"github.com/mmcdole/gofeed"
)

// FetchURL fetches the content of a URL and returns it as a byte slice
func FetchURL(url string, preferCache bool) []byte {
	fileStr := "izrss/" + URLToDir(url)
	file, err := xdg.CacheFile(fileStr)
	if err != nil {
		log.Fatal(err)
	}

	if data, errr := os.ReadFile(file); errr == nil && preferCache {
		return data
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(file, body, 0644)
	if err != nil {
		log.Fatal(err)
	}

	return body
}

// GetContentForURL fetches the content of a URL and returns it as a Feed
func GetContentForURL(url string, preferCache bool) Feed {
	if preferCache {
		if cachedFeed, found := cache.Get(url); found {
			return cachedFeed
		}
	}

	fp := gofeed.NewParser()
	feedFile := FetchURL(url, preferCache)

	if feedFile == nil {
		return Feed{
			Title: fmt.Sprintf("Error loading %s", url),
			URL:   url,
			Posts: []Post{},
		}
	}

	feed, err := fp.ParseString(string(feedFile))
	if err != nil {
		log.Fatalf("could not parse feed: %v", err)
	}

	newFeed := Feed{
		Title:       feed.Title,
		URL:         url,
		Posts:       []Post{},
		LastUpdated: time.Now(),
	}

	for _, item := range feed.Items {
		post := createPost(item)
		newFeed.Posts = append(newFeed.Posts, post)
	}

	cache.Set(url, newFeed, 24*time.Hour)

	return newFeed
}

// GetPosts fetches the content of a URL and returns it as a slice of Posts
func GetPosts(url string) []Post {
	feed := setupReader(url, false)
	posts := []Post{}

	if feed == nil {
		return posts
	}

	for _, item := range feed.Items {
		post := createPost(item)
		posts = append(posts, post)
	}

	return posts
}

func createPost(item *gofeed.Item) Post {
	content := ""
	if item.Content != "" {
		content = item.Content
	} else if item.Description != "" {
		content = item.Description
	} else {
		content = "This post does not contain any content.\nPress \"o\" to open the post in your preferred browser"
	}

	post := Post{
		Title:   item.Title,
		Content: content,
		Link:    item.Link,
		Date:    ConvertDate(item.Published),
		UUID:    item.GUID,
	}

	return post
}

func setupReader(url string, preferCache bool) *gofeed.Feed {
	fp := gofeed.NewParser()

	file := string(FetchURL(url, preferCache))

	if file == "" {
		return nil
	}

	feed, err := fp.ParseString(file)
	if err != nil {
		log.Fatalf("could not parse feed: %v", err)
	}

	return feed
}

// GetAllContent fetches the content of all URLs and returns it as a slice of Feeds
func GetAllContent(preferCache bool) Feeds {
	urls := ParseUrls()

	cache := NewCache()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Create a channel to receive responses
	responses := make(chan Feed)

	// Loop through the URLs and start a goroutine for each
	for _, url := range urls {
		wg.Add(1)
		go fetchContent(url, preferCache, &wg, responses)
	}

	// Close the responses channel when all goroutines are done
	go func() {
		wg.Wait()
		close(responses)
	}()

	feeds := Feeds{}
	for response := range responses {
		feeds = append(feeds, response)
	}

	return feeds.sort(urls)
}

func fetchContent(url string, preferCache bool, wg *sync.WaitGroup, ch chan<- Feed) {
	// Call the GetContentForURL function
	posts := GetContentForURL(url, preferCache)

	// Decrement the wait group counter when the function exits
	defer wg.Done()

	// Send the response through the channel
	ch <- posts
}

// CacheEntry represents a cached item with its expiration time
type CacheEntry struct {
	Expiration time.Time
	Value      Post
}

// Cache represents a cache with a map to store cached items
type Cache struct {
	data map[string]CacheEntry
	dir  string
	mu   sync.RWMutex
}

// NewCache creates a new instance of Cache
func NewCache() *Cache {
	cacheDir := "cache" // Default cache directory
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		panic(fmt.Sprintf("failed to create cache directory: %v", err))
	}
	return &Cache{
		data: make(map[string]CacheEntry),
		dir:  cacheDir,
	}
}

// Get retrieves a value from the cache by key
func (c *Cache) Get(key string) (Post, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.data[key]
	if !found {
		return Post{}, false
	}

	// Check if the entry has expired
	if time.Now().After(entry.Expiration) {
		// If expired, delete the entry from the cache
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		return Post{}, false
	}

	return entry.Value, true
}

// Set adds or updates a value in the cache with a specified expiration time
func (c *Cache) Set(key string, value Post, expiration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = CacheEntry{
		Value:      value,
		Expiration: time.Now().Add(expiration),
	}

	// Save the cache entry to a file
	err := c.saveToFile(key, value)
	if err != nil {
		panic(err)
	}
}

// saveToFile saves a cache entry to a file in the cache directory
func (c *Cache) saveToFile(key string, value Post) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	filename := filepath.Join(c.dir, key+".json")
	return os.WriteFile(filename, data, 0644)
}

// LoadCache loads cache entries from files in the cache directory
func (c *Cache) LoadCache() error {
	files, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			filename := filepath.Join(c.dir, file.Name())
			data, err := os.ReadFile(filename)
			if err != nil {
				return err
			}

			var post Post
			if err := json.Unmarshal(data, &post); err != nil {
				return err
			}

			c.Set(post.UUID, post, time.Until(post.Expiration))
		}
	}

	return nil
}
