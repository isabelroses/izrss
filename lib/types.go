package lib

type Post struct {
	Title   string
	Content string
	Link    string
	Date    string
}

type Feed struct {
	Title string
	URL   string
	Posts []Post
}

type Feeds []Feed
