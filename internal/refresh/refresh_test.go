package refresh

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/isabelroses/izrss/internal/rss"
)

var now = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

func urls(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("https://f%d.example", i)
	}
	return out
}

func setOf(us []string) *rss.FeedSet {
	var fs []rss.Feed
	for _, u := range us {
		fs = append(fs, rss.Feed{URL: u, Title: "T " + u, Posts: []rss.Post{{UUID: "old"}}})
	}
	return rss.NewFeedSet(fs...)
}

func TestProgressNeverExceedsDistinctFeeds(t *testing.T) {
	us := urls(5)
	r := New(setOf(us))
	if got := r.Start(us[:2]); len(got) != 2 {
		t.Fatalf("Start fetched %d, want 2", len(got))
	}
	if got := r.Start(us); len(got) != 3 { // r then R: adds only the rest
		t.Errorf("overlapping Start fetched %d, want 3", len(got))
	}
	if got := r.Start(us); len(got) != 0 {
		t.Errorf("Start while all in flight fetched %d", len(got))
	}
	if _, total := r.Progress(); total != 5 {
		t.Errorf("total %d, want 5", total)
	}
}

// R while r runs must not refetch what r finished (the total was 337 of 256).
func TestStartSkipsFeedsFinishedThisRound(t *testing.T) {
	us := urls(256)
	r := New(setOf(us))
	r.Start(us[:81])
	for _, u := range us[:40] {
		r.Finish(rss.Feed{URL: u}, now)
	}
	got := r.Start(us)
	if len(got) != 256-81 {
		t.Errorf("R fetched %d feeds, want %d (not the 40 finished nor the 41 in flight)", len(got), 256-81)
	}
	if done, total := r.Progress(); done != 40 || total != 256 {
		t.Errorf("progress %d/%d, want 40/256", done, total)
	}
}

func TestRoundEndsAndNextStartsFresh(t *testing.T) {
	us := urls(2)
	r := New(setOf(us))
	r.Start(us)
	if res := r.Finish(rss.Feed{URL: us[0]}, now); res.Finished {
		t.Error("the round isn't over after one of two")
	}
	res := r.Finish(rss.Feed{URL: us[1]}, now)
	if !res.Finished || !r.At().Equal(now) {
		t.Errorf("finished %v at %v", res.Finished, r.At())
	}
	if _, total := r.Progress(); total != 0 {
		t.Errorf("idle refresher reports total %d", total)
	}
	if got := r.Start(us[:1]); len(got) != 1 {
		t.Errorf("a new round should fetch again, got %d", len(got))
	}
}

func TestUnrequestedAndDuplicateResultsAreNotProgress(t *testing.T) {
	us := urls(2)
	r := New(setOf(us))
	r.Start(us)
	r.Finish(rss.Feed{URL: us[0]}, now)
	r.Finish(rss.Feed{URL: us[0]}, now) // late duplicate
	r.Finish(rss.Feed{URL: "https://other.example"}, now)
	if done, total := r.Progress(); done != 1 || total != 2 {
		t.Errorf("progress %d/%d, want 1/2", done, total)
	}
}

func TestFinishAppliesToFeedSet(t *testing.T) {
	us := urls(2)
	set := setOf(us)
	r := New(set)
	r.Start(us)

	res := r.Finish(rss.Feed{URL: us[1], Title: "New", Posts: []rss.Post{{UUID: "n1"}, {UUID: "n2"}}}, now)
	if !res.Changed || res.Err != nil {
		t.Fatalf("result %+v", res)
	}
	if _, ok := set.Post(rss.PostRef{FeedURL: us[1], Key: "n2"}); !ok {
		t.Error("the new posts should be in the set")
	}
	if got := set.URLs(); got[0] != us[0] || got[1] != us[1] {
		t.Errorf("order changed: %v", got)
	}
}

func TestFailedFetchKeepsFeedAndNamesTheKnownOne(t *testing.T) {
	us := urls(1)
	set := setOf(us)
	r := New(set)
	r.Start(us)

	res := r.Finish(rss.Feed{URL: us[0], Title: "f0.example", Err: errors.New("boom")}, now)
	if res.Changed || res.Err == nil {
		t.Errorf("result %+v", res)
	}
	if res.Name != "T "+us[0] {
		t.Errorf("name %q, want the known feed's title", res.Name)
	}
	if f, _ := set.Feed(us[0]); len(f.Posts) != 1 {
		t.Error("a failed refresh must keep the cached posts")
	}
	if !res.Finished {
		t.Error("a failure still finishes the round")
	}
}

func TestSetAtKeepsKnownTimeOnZero(t *testing.T) {
	r := New(setOf(nil))
	r.SetAt(now)
	r.SetAt(time.Time{})
	if !r.At().Equal(now) {
		t.Errorf("At = %v", r.At())
	}
}
