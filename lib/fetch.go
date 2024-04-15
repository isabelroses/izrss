package lib

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

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

func GetContentForURL(url string) Feed {
	feed := setupReader(url)

	if feed == nil {
		return Feed{
			fmt.Sprintf("Error loading %s", url),
			[]Post{},
			url,
		}
	}

	feedRet := Feed{
		feed.Title,
		[]Post{},
		url,
	}

	// could be deduplicated but unsure what the best way to do that is
	for _, item := range feed.Items {
		post := createPost(item)

		feedRet.Posts = append(feedRet.Posts, post)
	}

	return feedRet
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

// go reotines am irite
func GetAllContent() Feeds {
	urls := ParseUrls()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Create a channel to receive responses
	responses := make(chan Feed)

	// Loop through the URLs and start a goroutine for each
	for _, url := range urls {
		wg.Add(1)
		go fetchContent(url, &wg, responses)
	}

	// Close the responses channel when all goroutines are done
	go func() {
		wg.Wait()
		close(responses)
	}()

	feeds := Feeds{}
	for response := range responses {
		feeds = append(feeds, response)
	}

	return feeds
}

func fetchContent(url string, wg *sync.WaitGroup, ch chan<- Feed) {
	// Decrement the wait group counter when the function exits
	defer wg.Done()

	// Call the GetContentForURL function
	posts := GetContentForURL(url)

	// Send the response through the channel
	ch <- posts
}
