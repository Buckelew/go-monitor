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
	var allItems []monitor.Item
	var totalDuration time.Duration
	var totalBodySize int
	var lastCache string

	for page := 1; ; page++ {
		if err := s.client.RotateProxy(); err != nil {
			return nil, fmt.Errorf("failed to rotate proxy: %w", err)
		}

		url := productsURL(s.baseURL, page)
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

		if page == 1 {
			if resp.StatusCode == 401 {
				resp.Body.Close()
				return nil, monitor.ErrPasswordPage
			}
			lastCache = cacheStatus(resp)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, s.URL)
		}

		// Stream-parse directly from the response body instead of io.ReadAll,
		// avoiding a full-body []byte allocation per page.
		cr := &monitor.CountingReader{R: resp.Body}
		items, err := parseItems(cr, s.URL)
		io.Copy(io.Discard, cr) // drain remaining bytes for accurate count + connection reuse
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		totalBodySize += cr.N

		allItems = append(allItems, items...)

		// Less than 250 means we've reached the last page
		if len(items) < 250 {
			break
		}
	}

	return &monitor.FetchResult{
		Items: allItems,
		Meta: monitor.FetchMeta{
			StatusCode:  200,
			CacheStatus: lastCache,
			Duration:    totalDuration,
			BodySize:    totalBodySize,
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
