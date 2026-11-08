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
	Published time.Time `json:"published"` // zero when the feed's date couldn't be read
	// RawDate is the feed's own date text, kept only when it couldn't be read.
	RawDate string `json:"raw_date,omitempty"`
	UUID    string `json:"guid,omitempty"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Link    string `json:"link,omitempty"`
}

type Feed struct {
	Title   string
	URL     string
	Posts   []Post
	Fetched time.Time // when the feed was fetched; zero if never

	Err      error // the feed could not be fetched or parsed
	CacheErr error // the feed loaded but could not be cached
}

// Date is when p was published, in layout, or the feed's own date text when
// it couldn't be read.
func (p Post) Date(layout string) string {
	if p.Published.IsZero() {
		return p.RawDate
	}
	return p.Published.Format(layout)
}

// keyDateLayout dates the posts identified by title and date. It is fixed,
// not the configured date format, so changing that keeps them; it is the old
// default format, so posts saved before keep their keys.
const keyDateLayout = "02/01/2006"

// Key identifies a post: its GUID, else its link, else title and date.
func (p Post) Key() string {
	switch {
	case p.UUID != "":
		return p.UUID
	case p.Link != "":
		return p.Link
	}
	return p.Title + "\x00" + p.Date(keyDateLayout)
}

// StatusError is a feed server's reply other than 200 OK.
type StatusError struct {
	Code   int
	Status string // as "404 Not Found"
}

func (e *StatusError) Error() string { return e.Status }

// ParseError is a feed that was fetched but could not be parsed.
type ParseError struct{ Err error }

func (e *ParseError) Error() string { return e.Err.Error() }
func (e *ParseError) Unwrap() error { return e.Err }

// Fetcher fetches feeds over the network, parses them and caches them in a
// Store. Its fetches share one concurrency limit, however many callers there
// are.
type Fetcher struct {
	store *Store
	sem   chan struct{}
}

func NewFetcher(store *Store) *Fetcher {
	return &Fetcher{store: store, sem: make(chan struct{}, maxConcurrentFetches)}
}

// Fetch fetches, parses and caches the feed at url. It never fails outright:
// a feed that can't be loaded comes back empty with Err set, and one that
// loads but can't be cached comes back with CacheErr set.
func (f *Fetcher) Fetch(url string) Feed {
	data, err := f.get(url)
	if err == nil {
		var feed Feed
		if feed, err = parse(url, data); err == nil {
			feed.Fetched = time.Now()
			if err := f.store.Save(feed); err != nil {
				feed.CacheErr = fmt.Errorf("caching %s: %w", url, err)
			}
			return feed
		}
	}
	return failed(url, err)
}

// FetchAll fetches every URL concurrently (bounded by the Fetcher) and
// returns the feeds in URL order.
func (f *Fetcher) FetchAll(urls []string) []Feed { return fetchAll(urls, f.Fetch) }

// get is the body of the document at url.
func (f *Fetcher) get(url string) ([]byte, error) {
	f.sem <- struct{}{}
	defer func() { <-f.sem }()

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: %w", url, &StatusError{Code: resp.StatusCode, Status: resp.Status})
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", url, err)
	}
	if int64(len(body)) > maxFeedBytes {
		return nil, fmt.Errorf("reading %s: feed is larger than %d MiB", url, maxFeedBytes>>20)
	}
	return body, nil
}

// failed is the empty feed at url that err stopped loading.
func failed(url string, err error) Feed { return Feed{URL: url, Posts: []Post{}, Err: err} }

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
func sortedPosts(feed *gofeed.Feed) []Post {
	posts := make([]Post, 0, len(feed.Items))
	for _, item := range feed.Items {
		posts = append(posts, createPost(item))
	}
	slices.SortStableFunc(posts, func(a, b Post) int { return b.Published.Compare(a.Published) })
	return posts
}

func createPost(item *gofeed.Item) Post {
	content := item.Content
	if content == "" {
		content = item.Description
	}
	if content == "" {
		content = "This post does not contain any content.\nPress \"o\" to open the post in your preferred browser"
	}

	post := Post{
		Title:     cleanText(item.Title),
		Content:   content,
		Link:      item.Link,
		Published: parseDate(item),
		UUID:      item.GUID,
	}
	if post.Published.IsZero() {
		post.RawDate = cleanText(item.Published)
	}
	return post
}

// newParser is a gofeed parser that skips the RSS translator's search of
// every item's HTML for a first <img>: a full HTML parse per item, for an
// Item.Image izrss never reads.
func newParser() *gofeed.Parser {
	p := gofeed.NewParser()
	p.RSSTranslator = &gofeed.DefaultRSSTranslator{DisableContentImageScan: true}
	return p
}

// parse is the feed at url whose document is data.
func parse(url string, data []byte) (Feed, error) {
	if len(data) == 0 {
		return Feed{}, fmt.Errorf("%s returned an empty feed", url)
	}
	feed, err := newParser().Parse(bytes.NewReader(data))
	if err != nil {
		return Feed{}, fmt.Errorf("parsing %s: %w", url, &ParseError{err})
	}
	return Feed{Title: cleanText(feed.Title), URL: url, Posts: sortedPosts(feed)}, nil
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

// parseDate is an item's publish time, preferring gofeed's pre-parsed
// timestamp and falling back to manual layouts; zero if neither reads it.
func parseDate(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, item.Published); err == nil {
			return t
		}
	}
	return time.Time{}
}
