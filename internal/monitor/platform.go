package monitor

import (
	"fmt"
	"net/url"
	"strings"
)

// DetectPlatform infers the monitoring platform from a URL's hostname.
func DetectPlatform(rawURL string) (Platform, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.ToLower(u.Hostname())

	switch {
	case strings.HasSuffix(host, ".myshopify.com"):
		return Shopify, nil
	case strings.HasSuffix(host, ".bigcartel.com"):
		return Bigcartel, nil
	case strings.HasSuffix(host, ".squarespace.com"):
		return Squarespace, nil
	case host == "reddit.com" || host == "www.reddit.com" || host == "old.reddit.com":
		if strings.HasPrefix(u.Path, "/r/") {
			return Reddit, nil
		}
		return "", fmt.Errorf("reddit URL must be a subreddit (/r/...)")
	default:
		return "", fmt.Errorf("could not detect platform for host %q", host)
	}
}
