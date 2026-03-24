package reddit

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/buckelew/go-monitor/internal/monitor"
)

func postsURL(subredditURL, apiBase string) (string, error) {
	u, err := url.Parse(subredditURL)
	if err != nil {
		return "", fmt.Errorf("invalid subreddit URL: %w", err)
	}

	// Extract subreddit name from path like "/r/golang" or "/r/golang/"
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "r" {
		return "", fmt.Errorf("could not extract subreddit from %s", subredditURL)
	}
	subreddit := parts[1]

	base := "https://www.reddit.com"
	if apiBase != "" {
		base = strings.TrimRight(apiBase, "/")
	}
	return fmt.Sprintf("%s/r/%s/new.json?limit=100", base, subreddit), nil
}

func parseItems(r io.Reader) ([]monitor.Item, error) {
	var response redditResponse
	if err := json.NewDecoder(r).Decode(&response); err != nil {
		return nil, err
	}

	var items []monitor.Item
	for _, child := range response.Data.Children {
		post := child.Data

		var imageURL string
		switch post.Thumbnail {
		case "self", "default", "nsfw", "":
			// No usable thumbnail
		default:
			imageURL = post.Thumbnail
		}

		raw, err := json.Marshal(post)
		if err != nil {
			return nil, err
		}

		items = append(items, monitor.Item{
			URL:      "https://www.reddit.com" + post.Permalink,
			Title:    post.Title,
			InStock:  true,
			ImageURL: imageURL,
			Data:     raw,
		})
	}

	return items, nil
}
