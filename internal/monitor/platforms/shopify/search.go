package shopify

import (
	"context"
	"fmt"
	"io"
	"time"

	http "github.com/bogdanfinn/fhttp"

	"github.com/buckelew/go-monitor/internal/httpclient"
	"github.com/buckelew/go-monitor/internal/monitor"
)

type SearchShopify struct {
	URL     string
	baseURL string
	client  *httpclient.Client
}

func (s *SearchShopify) Platform() monitor.Platform {
	return monitor.Shopify
}

func (s *SearchShopify) Client() *httpclient.Client { return s.client }

func (s *SearchShopify) FetchProducts(ctx context.Context) (*monitor.FetchResult, error) {
	url := productsURL(s.baseURL)

	if err := s.client.RotateProxy(); err != nil {
		return nil, fmt.Errorf("failed to rotate proxy: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	resp, err := s.client.Inner().Do(req)
	duration := time.Since(start)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, monitor.ErrPasswordPage
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, s.URL)
	}

	readBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	items, err := parseItems(readBytes, s.URL)
	if err != nil {
		return nil, err
	}

	return &monitor.FetchResult{
		Items: items,
		Meta: monitor.FetchMeta{
			StatusCode:  resp.StatusCode,
			CacheStatus: cacheStatus(resp),
			Duration:    duration,
			BodySize:    len(readBytes),
			Proxy:       s.client.CurrentProxyRaw(),
		},
	}, nil
}

func cacheStatus(resp *http.Response) string {
	if v := resp.Header.Get("X-Cache"); v != "" {
		return v
	}
	return resp.Header.Get("Cf-Cache-Status")
}

func NewSearchShopify(URL string, client *httpclient.Client, opts ...*monitor.Opts) *SearchShopify {
	baseURL := URL
	if len(opts) > 0 && opts[0] != nil && opts[0].BaseURL != "" {
		baseURL = opts[0].BaseURL
	}
	return &SearchShopify{URL: URL, baseURL: baseURL, client: client}
}
