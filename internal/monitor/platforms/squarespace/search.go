package squarespace

import (
	"context"
	"fmt"
	"io"
	"time"

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

func (s *SearchSquarespace) Client() *httpclient.Client { return s.client }

func (s *SearchSquarespace) FetchProducts(ctx context.Context) (*monitor.FetchResult, error) {
	var allItems []monitor.Item
	var totalDuration time.Duration
	var totalBodySize int
	var lastProxy string
	offset := 0

	for {
		if err := s.client.RotateProxy(); err != nil {
			return nil, fmt.Errorf("failed to rotate proxy: %w", err)
		}
		lastProxy = s.client.CurrentProxyRaw()

		url := productsURL(s.baseURL, offset)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		start := time.Now()
		resp, err := s.client.Inner().Do(req)
		totalDuration += time.Since(start)
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
		totalBodySize += len(body)

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

	return &monitor.FetchResult{
		Items: allItems,
		Meta: monitor.FetchMeta{
			StatusCode: 200,
			Duration:   totalDuration,
			BodySize:   totalBodySize,
			Proxy:      lastProxy,
		},
	}, nil
}

func NewSearchSquarespace(URL string, client *httpclient.Client, opts ...*monitor.Opts) *SearchSquarespace {
	baseURL := URL
	if len(opts) > 0 && opts[0] != nil && opts[0].BaseURL != "" {
		baseURL = opts[0].BaseURL
	}
	return &SearchSquarespace{URL: URL, baseURL: baseURL, client: client}
}
