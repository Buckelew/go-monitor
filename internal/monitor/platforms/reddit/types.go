package reddit

type redditResponse struct {
	Data redditData `json:"data"`
}

type redditData struct {
	Children []redditChild `json:"children"`
	After    *string       `json:"after"`
}

type redditChild struct {
	Data redditPost `json:"data"`
}

type redditPost struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Author     string  `json:"author"`
	Permalink  string  `json:"permalink"`
	URL        string  `json:"url"`
	Thumbnail  string  `json:"thumbnail"`
	CreatedUTC float64 `json:"created_utc"`
	IsSelf     bool    `json:"is_self"`
	Score      int     `json:"score"`
}
