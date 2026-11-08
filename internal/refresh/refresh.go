// Package refresh runs network refreshes: which feeds are being fetched, how
// far along it is, how overlapping requests merge, and what a finished fetch
// does to the feeds. Fetches come from a Source; Start returns them as jobs
// for the caller to run off its main loop, handing each result to Finish.
package refresh

import (
	"time"

	"github.com/isabelroses/izrss/internal/rss"
)

// Source fetches one feed. A fetch never fails outright: a feed that can't
// be loaded comes back with Err set.
type Source interface {
	Fetch(url string) rss.Feed
}

// Feeds is what a refresh updates.
type Feeds interface {
	Feed(url string) (rss.Feed, bool)
	Replace(rss.Feed)
}

// Job is one fetch to run off the main loop, then report with Finish.
type Job func() rss.Feed

// Refresher refreshes one set of feeds. A round starts with the first Start
// while idle and ends when every feed asked for in it has finished; requests
// made meanwhile join the round.
type Refresher struct {
	feeds    Feeds
	src      Source
	inFlight map[string]bool // fetches started and not yet finished
	finished map[string]bool // feeds finished in this round
	at       time.Time       // when the last round finished
}

// New refreshes feeds from src. With no source, nothing is ever fetched.
func New(feeds Feeds, src Source) *Refresher {
	return &Refresher{feeds: feeds, src: src, inFlight: map[string]bool{}, finished: map[string]bool{}}
}

// Start asks for the feeds with urls to be refreshed and returns the fetches
// to run now. A feed already being fetched, or already refreshed in this
// round, is left out, so overlapping requests (r then R) never fetch a feed
// twice and progress never counts past the number of distinct feeds.
func (r *Refresher) Start(urls []string) (jobs []Job) {
	if r.src == nil {
		return nil
	}
	src := r.src
	for _, url := range urls {
		if r.inFlight[url] || r.finished[url] {
			continue
		}
		r.inFlight[url] = true
		jobs = append(jobs, func() rss.Feed { return src.Fetch(url) })
	}
	return jobs
}

// Result is what one finished fetch came to.
type Result struct {
	// Name is how the feed is called in messages: the known feed's name, even
	// if the result is a failure that only knows its URL.
	Name     string
	Err      error // the fetch failed; the feed set is unchanged
	CacheErr error // the feed loaded but could not be cached
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
		}
	}
	if f.Err == nil {
		r.feeds.Replace(f)
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
