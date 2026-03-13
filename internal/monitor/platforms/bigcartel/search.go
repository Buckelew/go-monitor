package bigcartel

import (
	"context"
	"fmt"
	"io"

	http "github.com/bogdanfinn/fhttp"

	"github.com/buckelew/go-monitor/internal/httpclient"
	"github.com/buckelew/go-monitor/internal/monitor"
)

type SearchBigCartel struct {
	URL     string
	baseURL string
	client  *httpclient.Client
}

func (s *SearchBigCartel) Platform() monitor.Platform {
	return monitor.Bigcartel
}

func (s *SearchBigCartel) FetchProducts(ctx context.Context) ([]monitor.Item, error) {
	apiURL, err := productsURL(s.URL, s.baseURL)
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

	return parseItems(body, s.URL)
}

func NewSearchBigCartel(URL string, client *httpclient.Client, opts ...*monitor.Opts) *SearchBigCartel {
	var baseURL string
	if len(opts) > 0 && opts[0] != nil && opts[0].BaseURL != "" {
		baseURL = opts[0].BaseURL
	}
	return &SearchBigCartel{URL: URL, baseURL: baseURL, client: client}
}
