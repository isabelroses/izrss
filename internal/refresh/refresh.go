// Package refresh is the bookkeeping of a network refresh: which feeds are
// being fetched, how far along it is, how overlapping requests merge, and
// what a finished fetch does to the feed set. It does the fetching nowhere:
// the caller fetches what Start returns and hands each result to Finish.
package refresh

import (
	"time"

	"github.com/isabelroses/izrss/internal/rss"
)

// Refresher tracks refreshing the feeds of one feed set. A round starts with
// the first Start while idle and ends when every feed asked for in it has
// finished; requests made meanwhile join the round.
type Refresher struct {
	feeds    *rss.FeedSet
	inFlight map[string]bool // fetches started and not yet finished
	finished map[string]bool // feeds finished in this round
	at       time.Time       // when the last round finished
}

// New refreshes the feeds of set.
func New(set *rss.FeedSet) *Refresher {
	return &Refresher{feeds: set, inFlight: map[string]bool{}, finished: map[string]bool{}}
}

// Start asks for the feeds with urls to be refreshed and returns those to
// fetch now. A feed already being fetched, or already refreshed in this
// round, is left out, so overlapping requests (r then R) never fetch a feed
// twice and progress never counts past the number of distinct feeds.
func (r *Refresher) Start(urls []string) (fetch []string) {
	for _, url := range urls {
		if r.inFlight[url] || r.finished[url] {
			continue
		}
		r.inFlight[url] = true
		fetch = append(fetch, url)
	}
	return fetch
}

// Result is what one finished fetch came to.
type Result struct {
	// Name is how the feed is called in messages: the known feed's name, even
	// if the result is a failure that only knows its URL.
	Name     string
	Err      error // the fetch failed; the feed set is unchanged
	CacheErr error // the feed loaded but could not be cached
	// Changed means the feed set now holds the feed's new posts.
	Changed bool
	// Finished means this result completed the round.
	Finished bool
}

// Finish takes one feed's fetched result: it counts toward progress if it
// was asked for, and replaces the feed in the set unless it failed.
func (r *Refresher) Finish(f rss.Feed, now time.Time) Result {
	res := Result{Name: f.Name(), Err: f.Err, CacheErr: f.CacheErr}
	if known, ok := r.feeds.Feed(f.URL); ok {
		res.Name = known.Name()
	}
	if r.inFlight[f.URL] { // a result not asked for isn't progress
		delete(r.inFlight, f.URL)
		r.finished[f.URL] = true
		if len(r.inFlight) == 0 {
			r.finished = map[string]bool{}
			r.at = now
			res.Finished = true
		}
	}
	if f.Err == nil {
		r.feeds.Replace(f)
		res.Changed = true
	}
	return res
}

// Progress is how many of the round's feeds have finished, out of how many;
// total is 0 when no round is running.
func (r *Refresher) Progress() (done, total int) {
	if len(r.inFlight) == 0 {
		return 0, 0
	}
	return len(r.finished), len(r.finished) + len(r.inFlight)
}

// At is when the last round finished, or when it was last known to have.
func (r *Refresher) At() time.Time { return r.at }

// SetAt records when a round finished before this run, if later than known.
func (r *Refresher) SetAt(at time.Time) {
	if !at.IsZero() {
		r.at = at
	}
}
