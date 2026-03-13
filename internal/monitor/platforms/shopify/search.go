package shopify

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	http "github.com/bogdanfinn/fhttp"

	"github.com/buckelew/go-monitor/internal/httpclient"
	"github.com/buckelew/go-monitor/internal/monitor"
)

type SearchShopify struct {
	URL        string
	baseURL    string
	client     *httpclient.Client
	requestNum atomic.Uint64
}

func (s *SearchShopify) Platform() monitor.Platform {
	return monitor.Shopify
}

func (s *SearchShopify) FetchProducts(ctx context.Context) ([]monitor.Item, error) {
	reqNum := s.requestNum.Add(1) - 1
	url := productsURL(s.baseURL, reqNum)

	if err := s.client.RotateProxy(); err != nil {
		return nil, fmt.Errorf("failed to rotate proxy: %w", err)
	}

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

	readBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	items, err := parseItems(readBytes, s.URL)
	if err != nil {
		return nil, err
	}

	return items, nil
}

func NewSearchShopify(URL string, client *httpclient.Client, opts ...*monitor.Opts) *SearchShopify {
	baseURL := URL
	if len(opts) > 0 && opts[0] != nil && opts[0].BaseURL != "" {
		baseURL = opts[0].BaseURL
	}
	return &SearchShopify{URL: URL, baseURL: baseURL, client: client}
}
