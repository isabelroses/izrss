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
	Read      bool
}

type Feed struct {
	Title string
	URL   string
	Posts []Post

	Err      error // the feed could not be fetched or parsed
	CacheErr error // the feed loaded but could not be cached
}

type Feeds []Feed

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

// Loc addresses a post as feeds[Feed].Posts[Post].
type Loc struct{ Feed, Post int }

// Locate finds the post with key, preferring the copy in the feed with
// feedURL (posts can be cross-posted); feedURL "" searches every feed.
func (f Feeds) Locate(feedURL, key string) (Loc, bool) {
	var fallback *Loc
	for i, feed := range f {
		for j, p := range feed.Posts {
			if p.Key() != key {
				continue
			}
			if feedURL == "" || feed.URL == feedURL {
				return Loc{Feed: i, Post: j}, true
			}
			if fallback == nil {
				fallback = &Loc{Feed: i, Post: j}
			}
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return Loc{}, false
}

// SetRead sets the read state of the post at loc.
func (f Feeds) SetRead(loc Loc, read bool) {
	f[loc.Feed].Posts[loc.Post].Read = read
}

// WritePostTrackingAt persists the read state of the post at loc.
func (f Feeds) WritePostTrackingAt(db *storage.DB, loc Loc) error {
	post := f[loc.Feed].Posts[loc.Post]
	return db.SavePostReadStatus(f[loc.Feed].URL, post.Key(), post.Read)
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

// SortPosts sorts posts by publish date in descending order
func SortPosts(posts []Post) {
	slices.SortStableFunc(posts, func(a, b Post) int { return b.Published.Compare(a.Published) })
}

// ReadAll marks all posts in a feed as read
func ReadAll(feeds Feeds, feedID int) {
	for i := range feeds[feedID].Posts {
		feeds[feedID].Posts[i].Read = true
	}
}

// WriteTracking saves the tracking state to the database
func (f Feeds) WriteTracking(db *storage.DB) error {
	total := 0
	for _, feed := range f {
		total += len(feed.Posts)
	}

	statuses := make([]storage.PostReadStatus, 0, total)
	for _, feed := range f {
		for _, post := range feed.Posts {
			statuses = append(statuses, storage.PostReadStatus{
				FeedURL: feed.URL,
				Key:     post.Key(),
				Read:    post.Read,
			})
		}
	}
	return db.SavePostReadStatuses(statuses)
}

// ReadTracking reads the tracking state from the database
func (f Feeds) ReadTracking(db *storage.DB) error {
	statuses, err := db.LoadPostReadStatuses()
	if err != nil {
		return err
	}
	for i := range f {
		for j, post := range f[i].Posts {
			if read, ok := statuses[storage.ReadKey{FeedURL: f[i].URL, Key: post.Key()}]; ok {
				f[i].Posts[j].Read = read
			}
		}
	}
	return nil
}

// ReadTrackingAt reads the tracking state of the feed at index i alone from
// the database, as ReadTracking would set it.
func (f Feeds) ReadTrackingAt(db *storage.DB, i int) error {
	statuses, err := db.LoadFeedReadStatuses(f[i].URL)
	if err != nil {
		return err
	}
	for j, post := range f[i].Posts {
		if read, ok := statuses[post.Key()]; ok {
			f[i].Posts[j].Read = read
		}
	}
	return nil
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

// FetchURL fetches the content of a URL. cacheErr reports a failure to cache
// a successful fetch, which does not stop the feed from loading.
func (f *Fetcher) FetchURL(url string, preferCache bool) (data []byte, cacheErr, err error) {
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
	feed, cacheErr, err := f.loadFeed(url, preferCache)
	if err != nil {
		return Feed{Title: hostOf(url), URL: url, Posts: []Post{}, Err: err, CacheErr: cacheErr}
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

// hostOf names a feed by its URL's host (and port) when it has no title.
func hostOf(raw string) string {
	if u, err := neturl.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

// sortedPosts converts a parsed feed's items into posts, newest first.
func (f *Fetcher) sortedPosts(feed *gofeed.Feed) []Post {
	posts := make([]Post, 0, len(feed.Items))
	for _, item := range feed.Items {
		posts = append(posts, f.createPost(item))
	}
	SortPosts(posts)
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

// loadFeed fetches and parses a feed; cacheErr is as for FetchURL.
func (f *Fetcher) loadFeed(url string, preferCache bool) (feed *gofeed.Feed, cacheErr, err error) {
	data, cacheErr, err := f.FetchURL(url, preferCache)
	if err != nil {
		return nil, cacheErr, err
	}
	feed, err = parseFeed(url, data)
	return feed, cacheErr, err
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
func (f *Fetcher) GetCachedContent(urls []string) Feeds {
	feeds := make(Feeds, len(urls))
	var wg sync.WaitGroup
	for i, url := range urls {
		wg.Go(func() { feeds[i] = f.cachedFeed(url) })
	}
	wg.Wait()
	return feeds
}

func (f *Fetcher) cachedFeed(url string) Feed {
	data, err := f.db.LoadFeedCache(url)
	if err == nil && data == nil {
		return Feed{Title: hostOf(url), URL: url, Posts: []Post{}}
	}
	var feed *gofeed.Feed
	if err == nil {
		feed, err = parseFeed(url, data)
	}
	if err != nil {
		return Feed{Title: hostOf(url), URL: url, Posts: []Post{}, Err: err}
	}
	return Feed{Title: cleanText(feed.Title), URL: url, Posts: f.sortedPosts(feed)}
}

// GetAllContent fetches every URL concurrently (bounded by the Fetcher)
// and returns the feeds in URL order. Failed feeds carry Err.
func (f *Fetcher) GetAllContent(urls []string, preferCache bool) Feeds {
	feeds := make(Feeds, len(urls))
	var wg sync.WaitGroup
	for i, url := range urls {
		wg.Go(func() { feeds[i] = f.GetContentForURL(url, preferCache) })
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
