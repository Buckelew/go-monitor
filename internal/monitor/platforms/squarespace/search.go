package squarespace

import (
	"context"
	"fmt"
	"io"

	http "github.com/bogdanfinn/fhttp"

	"github.com/buckelew/go-monitor/internal/httpclient"
	"github.com/buckelew/go-monitor/internal/monitor"
)

type SearchSquarespace struct {
	URL     string
	baseURL string
	client  *httpclient.Client
}

func (s *SearchSquarespace) Platform() monitor.Platform {
	return monitor.Squarespace
}

func (s *SearchSquarespace) FetchProducts(ctx context.Context) ([]monitor.Item, error) {
	var allItems []monitor.Item
	offset := 0

	for {
		if err := s.client.RotateProxy(); err != nil {
			return nil, fmt.Errorf("failed to rotate proxy: %w", err)
		}

		url := productsURL(s.baseURL, offset)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := s.client.Inner().Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, s.URL)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		items, err := parseItems(body, s.URL)
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, items...)

		next, nextOffset := hasNextPage(body)
		if !next {
			break
		}
		offset = nextOffset
	}

	return allItems, nil
}

func NewSearchSquarespace(URL string, client *httpclient.Client, opts ...*monitor.Opts) *SearchSquarespace {
	baseURL := URL
	if len(opts) > 0 && opts[0] != nil && opts[0].BaseURL != "" {
		baseURL = opts[0].BaseURL
	}
	return &SearchSquarespace{URL: URL, baseURL: baseURL, client: client}
}
