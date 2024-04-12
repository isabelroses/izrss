package lib

type Post struct {
	Title   string
	Content string
	Link    string
	Date    string
}

type Posts struct {
	Title string
	Posts []Post
	URL   string
}

type Feeds []Posts