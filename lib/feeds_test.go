package lib

import (
	"testing"
)

func TestMergeFeeds(t *testing.T) {
	// Test case 1: Feeds with no common posts
	feeds1 := Feeds{
		{Posts: []Post{
			{UUID: "1", Read: false},
			{UUID: "2", Read: true},
		}},
	}
	feeds2 := Feeds{
		{Posts: []Post{
			{UUID: "3", Read: false},
			{UUID: "4", Read: true},
		}},
	}

	feeds1.mergeFeeds(feeds2)

	// Ensure feeds1 remains unchanged
	if feeds1[0].Posts[0].Read != false || feeds1[0].Posts[1].Read != true {
		t.Errorf("Expected no change in feeds1")
	}

	// Ensure feeds2 remains unchanged
	if feeds2[0].Posts[0].Read != false || feeds2[0].Posts[1].Read != true {
		t.Errorf("Expected no change in feeds2")
	}

	// Test case 2: Feeds with common posts
	feeds3 := Feeds{
		{Posts: []Post{
			{UUID: "1", Read: false},
			{UUID: "2", Read: false},
		}},
	}
	feeds4 := Feeds{
		{Posts: []Post{
			{UUID: "1", Read: true},
			{UUID: "2", Read: true},
		}},
	}

	feeds3.mergeFeeds(feeds4)

	// Ensure that the "Read" state for posts in common gets merged
	if feeds3[0].Posts[0].Read == true && feeds3[0].Posts[1].Read == true {
		t.Errorf("Expected merged read states")
	}

	// Test case 3: Feeds with identical posts and same read state
	feeds5 := Feeds{
		{Posts: []Post{
			{UUID: "1", Read: true},
			{UUID: "2", Read: false},
		}},
	}
	feeds6 := Feeds{
		{Posts: []Post{
			{UUID: "1", Read: true},
			{UUID: "2", Read: false},
		}},
	}

	feeds5.mergeFeeds(feeds6)

	// Ensure that the feed remains unchanged as the states were the same
	if feeds5[0].Posts[0].Read != true || feeds5[0].Posts[1].Read != false {
		t.Errorf("Expected no change when posts have the same read state")
	}

	// Test case 4: Feeds with posts but no common posts
	feeds7 := Feeds{
		{Posts: []Post{
			{UUID: "1", Read: false},
			{UUID: "2", Read: false},
		}},
	}
	feeds8 := Feeds{
		{Posts: []Post{
			{UUID: "3", Read: true},
			{UUID: "4", Read: true},
		}},
	}

	feeds7.mergeFeeds(feeds8)

	// Ensure that the posts in feeds7 are not modified and posts in feeds8 are unaffected
	if feeds7[0].Posts[0].Read != false || feeds7[0].Posts[1].Read != false {
		t.Errorf("Expected no change in feeds7")
	}
	if feeds8[0].Posts[0].Read != true || feeds8[0].Posts[1].Read != true {
		t.Errorf("Expected no change in feeds8")
	}

	// Test case 4: Feeds with posts but no common posts
	feeds9 := Feeds{
		{Posts: []Post{
			{UUID: "1", Read: false},
			{UUID: "2", Read: false},
		}},
	}
	feeds10 := Feeds{
		{Posts: []Post{
			{UUID: "3", Read: true},
		}},
	}

	feeds9.mergeFeeds(feeds10)
}
