package lib

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/adrg/xdg"
	"github.com/mmcdole/gofeed"
)

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
		log.Fatal(err)
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

func URLToDir(url string) string {
	url = strings.ReplaceAll(url, "https://", "")
	url = strings.ReplaceAll(url, "http://", "")
	url = strings.ReplaceAll(url, "/", "_")
	url = strings.ReplaceAll(url, ".", "_")
	return url
}

func setupReader(url string) *gofeed.Feed {
	fp := gofeed.NewParser()

	file := string(FetchURL(url, true))

	feed, err := fp.ParseString(file)
	if err != nil {
		log.Fatalf("could not parse feed: %v", err)
	}

	return feed
}

func GetAllContent() Feeds {
	urls := ParseUrls()
	feeds := Feeds{}

	for _, url := range urls {
		posts := GetContentForURL(url)
		feeds = append(feeds, posts)
	}

	return feeds
}

func GetContentForURL(url string) Posts {
	feed := setupReader(url)

	postList := Posts{}
	postList.Title = feed.Title
	postList.Posts = []Post{}
	postList.URL = url

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
		Date:    ConvertDate(item.Published),
	}

	return post
}
