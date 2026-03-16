package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DetectPlatform infers the monitoring platform from a URL's hostname first,
// then probes the URL if hostname matching fails. A raw TCIN (all digits) is
// detected as Target.
func DetectPlatform(rawURL string) (Platform, error) {
	if regexp.MustCompile(`^\d+$`).MatchString(rawURL) {
		return Target, nil
	}

	platform, err := detectByHostname(rawURL)
	if err == nil {
		return platform, nil
	}

	// If hostname matched a known domain but validation failed (e.g. reddit
	// without /r/), return that error directly — don't probe.
	if strings.Contains(err.Error(), "must be") {
		return "", err
	}

	// Hostname didn't match — probe the URL
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return probeURL(ctx, rawURL)
}

// detectByHostname does fast suffix/exact matching on the hostname.
func detectByHostname(rawURL string) (Platform, error) {
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
	case host == "www.target.com" || host == "target.com":
		return Target, nil
	default:
		return "", fmt.Errorf("could not detect platform for host %q", host)
	}
}

// probeURL tries platform-specific endpoints to detect the platform.
func probeURL(ctx context.Context, rawURL string) (Platform, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	base := u.Scheme + "://" + u.Host

	client := &http.Client{Timeout: 10 * time.Second}

	// Try Shopify: GET /products.json
	if platform, ok := probeShopify(ctx, client, base); ok {
		return platform, nil
	}

	// Try Squarespace: GET /?format=json
	if platform, ok := probeSquarespace(ctx, client, base); ok {
		return platform, nil
	}

	return "", fmt.Errorf("could not detect platform for %q", rawURL)
}

func probeShopify(ctx context.Context, client *http.Client, base string) (Platform, bool) {
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/products.json?limit=1", nil)
	if err != nil {
		return "", false
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", false
	}

	var result struct {
		Products []json.RawMessage `json:"products"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", false
	}
	// Valid Shopify response has a "products" array
	if result.Products != nil {
		return Shopify, true
	}
	return "", false
}

func probeSquarespace(ctx context.Context, client *http.Client, base string) (Platform, bool) {
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/?format=json-pretty", nil)
	if err != nil {
		return "", false
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", false
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(body, &result); err != nil {
		return "", false
	}
	// Squarespace JSON responses contain "website" or "collection" keys
	if _, ok := result["website"]; ok {
		return Squarespace, true
	}
	if _, ok := result["collection"]; ok {
		return Squarespace, true
	}
	return "", false
}

var targetTCINRegex = regexp.MustCompile(`/A-(\d+)`)

// ExtractTargetTCIN extracts a TCIN from a Target product URL or a raw TCIN string.
func ExtractTargetTCIN(input string) (string, error) {
	// Raw TCIN (all digits)
	if regexp.MustCompile(`^\d+$`).MatchString(input) {
		return input, nil
	}

	matches := targetTCINRegex.FindStringSubmatch(input)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract TCIN from %q", input)
	}
	return matches[1], nil
}
