package reddit

import (
	"context"
	"fmt"
	"io"

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

func (s *SearchReddit) FetchProducts(ctx context.Context) ([]monitor.Item, error) {
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

	return parseItems(body)
}

func NewSearchReddit(URL string, client *httpclient.Client, opts ...*monitor.Opts) *SearchReddit {
	var baseURL string
	if len(opts) > 0 && opts[0] != nil && opts[0].BaseURL != "" {
		baseURL = opts[0].BaseURL
	}
	return &SearchReddit{URL: URL, baseURL: baseURL, client: client}
}
