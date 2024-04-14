package lib

import (
	"fmt"
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
		return nil
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

	if feed == nil {
		return Posts{
			fmt.Sprintf("Error loading %s", url),
			[]Post{},
			url,
		}
	}

	postList := Posts{
		feed.Title,
		[]Post{},
		url,
	}

	// could be deduplicated but unsure what the best way to do that is
	for _, item := range feed.Items {
		post := createPost(item)

		postList.Posts = append(postList.Posts, post)
	}

	return postList
}

func GetPosts(url string) []Post {
	feed := setupReader(url)

	posts := []Post{}

	for _, item := range feed.Items {
		post := createPost(item)

		posts = append(posts, post)
	}

	return posts
}

func createPost(item *gofeed.Item) Post {
	content := ""
	if item.Content != "" {
		content = item.Content
	} else {
		content = item.Description
	}

	post := Post{
		Title:   item.Title,
		Content: content,
		Link:    item.Link,
		Date:    ConvertDate(item.Published),
	}

	return post
}

func setupReader(url string) *gofeed.Feed {
	fp := gofeed.NewParser()

	file := string(FetchURL(url, true))

	if file == "" {
		return nil
	}

	feed, err := fp.ParseString(file)
	if err != nil {
		log.Fatalf("could not parse feed: %v", err)
	}

	return feed
}
