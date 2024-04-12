package cmd

import (
	"context"
	"log"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/isabelroses/izrss/lib"
)

type Post struct {
	Title   string
	Content string
	Link    string
	Date    string
}

type Posts struct {
	Title string
	Posts []Post
	Url   string
}

type Feeds []Posts

func setupReader(url string) *gofeed.Feed {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fp := gofeed.NewParser()
	fp.UserAgent = "izrss 0.0.1"

	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		log.Fatalf("could not parse feed: %v", err)
	}

	return feed
}

func GetAllContent() Feeds {
	urls := lib.ParseUrls()
	feeds := Feeds{}

	for _, url := range urls {
		posts := GetContentForUrl(url)
		feeds = append(feeds, posts)
	}

	return feeds
}

func GetContentForUrl(url string) Posts {
	feed := setupReader(url)

	postList := Posts{}
	postList.Title = feed.Title
	postList.Posts = []Post{}
	postList.Url = url

	for _, item := range feed.Items {
		post := createPost(item)

		postList.Posts = append(postList.Posts, post)
	}

	return postList
}

func createPost(item *gofeed.Item) Post {
	content := ""
	if item.Description != "" {
		content = item.Description
	} else {
		content = item.Content
	}

	post := Post{
		Title:   item.Title,
		Content: content,
		Link:    item.Link,
		Date:    lib.ConvertDate(item.Published),
	}

	return post
}
