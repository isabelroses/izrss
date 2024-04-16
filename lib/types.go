package lib

type Post struct {
	Title   string
	Content string
	Link    string
	Date    string
	ID      int
}

type Feed struct {
	Title string
	URL   string
	Posts []Post
	ID    int
}

type Feeds []Feed
