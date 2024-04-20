package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/isabelroses/izrss/lib"
)

// CacheEntry represents a cached item with its expiration time
type CacheEntry struct {
	Value      lib.Post  // Store Post object
	Expiration time.Time // Expiration time
}

// Cache represents a cache with a map to store cached items
type Cache struct {
	data map[string]CacheEntry
	dir  string
	mu   sync.RWMutex
}

// NewCache creates a new instance of Cache
func NewCache(dir string) *Cache {
	return &Cache{
		data: make(map[string]CacheEntry),
		dir:  dir,
	}
}

// Get retrieves a value from the cache by key
func (c *Cache) Get(key string) (lib.Post, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.data[key]
	if !found {
		return lib.Post{}, false
	}

	// Check if the entry has expired
	if time.Now().After(entry.Expiration) {
		// If expired, delete the entry from the cache
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		return lib.Post{}, false
	}

	return entry.Value, true
}

// Set adds or updates a value in the cache with a specified expiration time
func (c *Cache) Set(key string, value lib.Post, expiration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = CacheEntry{
		Value:      value,
		Expiration: time.Now().Add(expiration),
	}

	// Save the cache entry to a file
	c.saveToFile(key, value)
}

// saveToFile saves a cache entry to a file in the cache directory
func (c *Cache) saveToFile(key string, value lib.Post) error {
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

			var post lib.Post
			if err := json.Unmarshal(data, &post); err != nil {
				return err
			}

			c.Set(post.UUID, post, time.Until(post.Expiration))
		}
	}

	return nil
}
