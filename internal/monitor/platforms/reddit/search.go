package reddit

import (
	"context"
	"fmt"
	"io"
	"time"

	http "github.com/bogdanfinn/fhttp"

	"github.com/buckelew/go-monitor/internal/httpclient"
	"github.com/buckelew/go-monitor/internal/monitor"
)

const userAgent = "go-monitor:v1.0.0 (monitor bot)"

type SearchReddit struct {
	URL     string
	baseURL string
	client  *httpclient.Client
}

func (s *SearchReddit) Platform() monitor.Platform {
	return monitor.Reddit
}

func (s *SearchReddit) Client() *httpclient.Client { return s.client }

func (s *SearchReddit) FetchProducts(ctx context.Context) (*monitor.FetchResult, error) {
	apiURL, err := postsURL(s.URL, s.baseURL)
	if err != nil {
		return nil, err
	}

	if err := s.client.RotateProxy(); err != nil {
		return nil, fmt.Errorf("failed to rotate proxy: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	start := time.Now()
	resp, err := s.client.Inner().Do(req)
	duration := time.Since(start)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, s.URL)
	}

	cr := &monitor.CountingReader{R: resp.Body}
	items, err := parseItems(cr)
	io.Copy(io.Discard, cr) // drain remaining bytes for connection reuse
	if err != nil {
		return nil, err
	}

	return &monitor.FetchResult{
		Items: items,
		Meta: monitor.FetchMeta{
			StatusCode: resp.StatusCode,
			Duration:   duration,
			BodySize:   cr.N,
			Proxy:      s.client.CurrentProxyRaw(),
		},
	}, nil
}

func NewSearchReddit(URL string, client *httpclient.Client, opts ...*monitor.Opts) *SearchReddit {
	var baseURL string
	if len(opts) > 0 && opts[0] != nil && opts[0].BaseURL != "" {
		baseURL = opts[0].BaseURL
	}
	return &SearchReddit{URL: URL, baseURL: baseURL, client: client}
}
