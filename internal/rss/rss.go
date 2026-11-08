// Package rss handles RSS feed fetching and parsing
package rss

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/mmcdole/gofeed"

	"github.com/isabelroses/izrss/internal/storage"
)

// httpClient's timeout stops one hung feed from stalling a whole refresh.
var httpClient = &http.Client{Timeout: 20 * time.Second}

// maxConcurrentFetches bounds peak memory: each in-flight feed holds its full
// body and parsed object graph, so a large feed list would otherwise spike.
const maxConcurrentFetches = 12

// maxFeedBytes caps a feed body, far above any real feed, so a broken or
// hostile server can't exhaust memory. A var so tests can lower it.
var maxFeedBytes int64 = 32 << 20

type Post struct {
	Published time.Time
	UUID      string
	Title     string
	Content   string
	Link      string
	Date      string
}

type Feed struct {
	Title string
	URL   string
	Posts []Post

	Err      error // the feed could not be fetched or parsed
	CacheErr error // the feed loaded but could not be cached
}

// Key identifies a post: its GUID, else its link, else title and date.
func (p Post) Key() string {
	switch {
	case p.UUID != "":
		return p.UUID
	case p.Link != "":
		return p.Link
	}
	return p.Title + "\x00" + p.Date
}

// Fetcher handles RSS feed fetching. Its fetches share one concurrency
// limit, however many callers there are.
type Fetcher struct {
	db         *storage.DB
	dateFormat string
	sem        chan struct{}
}

func NewFetcher(db *storage.DB, dateFormat string) *Fetcher {
	return &Fetcher{
		db:         db,
		dateFormat: dateFormat,
		sem:        make(chan struct{}, maxConcurrentFetches),
	}
}

// fetch fetches the content of a URL. cacheErr reports a failure to cache a
// successful fetch, which does not stop the feed from loading.
func (f *Fetcher) fetch(url string, preferCache bool) (data []byte, cacheErr, err error) {
	if preferCache {
		if data, err := f.db.LoadFeedCache(url); err == nil && data != nil {
			return data, nil, nil
		}
	}

	f.sem <- struct{}{}
	defer func() { <-f.sem }()

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("fetching %s: %s", url, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", url, err)
	}
	if int64(len(body)) > maxFeedBytes {
		return nil, nil, fmt.Errorf("reading %s: feed is larger than %d MiB", url, maxFeedBytes>>20)
	}

	if err := f.db.SaveFeedCache(url, body); err != nil {
		cacheErr = fmt.Errorf("caching %s: %w", url, err)
	}
	return body, cacheErr, nil
}

// GetContentForURL fetches and parses a feed. It never fails outright: a
// feed that can't be loaded comes back empty with Err set.
func (f *Fetcher) GetContentForURL(url string, preferCache bool) Feed {
	data, cacheErr, err := f.fetch(url, preferCache)
	return f.parsed(url, data, cacheErr, err)
}

// parsed is the Feed for data got from url, or for err if that failed;
// cacheErr is as for fetch.
func (f *Fetcher) parsed(url string, data []byte, cacheErr, err error) Feed {
	var feed *gofeed.Feed
	if err == nil {
		feed, err = parseFeed(url, data)
	}
	if err != nil {
		return Feed{URL: url, Posts: []Post{}, Err: err, CacheErr: cacheErr}
	}
	return Feed{Title: cleanText(feed.Title), URL: url, Posts: f.sortedPosts(feed), CacheErr: cacheErr}
}

// cleanText makes untrusted feed text safe for one terminal line: control
// characters (escape sequences) are dropped, tabs and newlines become spaces.
func cleanText(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			return ' '
		case unicode.IsControl(r): // C0, DEL and C1
			return -1
		}
		return r
	}, s)
}

// Name is how the feed is shown, everywhere: its title, else its URL's host without
// "www.", else its URL.
func (f Feed) Name() string {
	if strings.TrimSpace(f.Title) != "" {
		return f.Title
	}
	if u, err := neturl.Parse(f.URL); err == nil && u.Host != "" {
		return strings.TrimPrefix(u.Host, "www.")
	}
	return f.URL
}

// sortedPosts converts a parsed feed's items into posts, newest first.
func (f *Fetcher) sortedPosts(feed *gofeed.Feed) []Post {
	posts := make([]Post, 0, len(feed.Items))
	for _, item := range feed.Items {
		posts = append(posts, f.createPost(item))
	}
	slices.SortStableFunc(posts, func(a, b Post) int { return b.Published.Compare(a.Published) })
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

	published, date := parseDate(item, f.dateFormat)

	return Post{
		Title:     cleanText(item.Title),
		Content:   content,
		Link:      item.Link,
		Date:      cleanText(date),
		Published: published,
		UUID:      item.GUID,
	}
}

// newParser is a gofeed parser that skips the RSS translator's search of
// every item's HTML for a first <img>: a full HTML parse per item, for an
// Item.Image izrss never reads.
func newParser() *gofeed.Parser {
	p := gofeed.NewParser()
	p.RSSTranslator = &gofeed.DefaultRSSTranslator{DisableContentImageScan: true}
	return p
}

func parseFeed(url string, data []byte) (*gofeed.Feed, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%s returned an empty feed", url)
	}
	feed, err := newParser().Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", url, err)
	}
	return feed, nil
}

// GetCachedContent returns the feeds from the cache alone, never touching the
// network, in URL order. A feed with nothing cached comes back empty and
// named by its host, without Err, for a later refresh to fill in.
func (f *Fetcher) GetCachedContent(urls []string) []Feed {
	return fetchAll(urls, func(url string) Feed {
		data, err := f.db.LoadFeedCache(url)
		if err == nil && data == nil {
			return Feed{URL: url, Posts: []Post{}}
		}
		return f.parsed(url, data, nil, err)
	})
}

// GetAllContent fetches every URL concurrently (bounded by the Fetcher)
// and returns the feeds in URL order. Failed feeds carry Err.
func (f *Fetcher) GetAllContent(urls []string, preferCache bool) []Feed {
	return fetchAll(urls, func(url string) Feed { return f.GetContentForURL(url, preferCache) })
}

// fetchAll runs get for every url concurrently, keeping URL order.
func fetchAll(urls []string, get func(url string) Feed) []Feed {
	feeds := make([]Feed, len(urls))
	var wg sync.WaitGroup
	for i, url := range urls {
		wg.Go(func() { feeds[i] = get(url) })
	}
	wg.Wait()
	return feeds
}

// dateLayouts are tried when gofeed could not parse an item's date. gofeed's
// own parser already covers every other common layout.
var dateLayouts = []string{
	"02-Jan-06 15:04:05 MST",
	"02/Jan/2006",
	"02-Jan-2006",
}

// parseDate returns an item's publish time (for sorting) and display string,
// preferring gofeed's pre-parsed timestamp and falling back to manual layouts.
func parseDate(item *gofeed.Item, dateFormat string) (time.Time, string) {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed, item.PublishedParsed.Format(dateFormat)
	}

	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, item.Published); err == nil {
			return t, t.Format(dateFormat)
		}
	}

	return time.Time{}, item.Published
}
