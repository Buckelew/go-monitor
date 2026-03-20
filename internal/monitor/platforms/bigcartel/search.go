package bigcartel

import (
	"context"
	"fmt"
	"io"
	"time"

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

func (s *SearchBigCartel) Client() *httpclient.Client { return s.client }

func (s *SearchBigCartel) FetchProducts(ctx context.Context) (*monitor.FetchResult, error) {
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

	start := time.Now()
	resp, err := s.client.Inner().Do(req)
	duration := time.Since(start)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return nil, monitor.ErrPasswordPage
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, s.URL)
	}

	cr := &monitor.CountingReader{R: resp.Body}
	items, err := parseItems(cr, s.URL)
	io.Copy(io.Discard, cr) // drain remaining bytes for connection reuse
	if err != nil {
		return nil, err
	}

	return &monitor.FetchResult{
		Items: items,
		Meta: monitor.FetchMeta{
			StatusCode:  resp.StatusCode,
			Duration:    duration,
			BodySize:    cr.N,
			Proxy:       s.client.CurrentProxyRaw(),
		},
	}, nil
}

func NewSearchBigCartel(URL string, client *httpclient.Client, opts ...*monitor.Opts) *SearchBigCartel {
	var baseURL string
	if len(opts) > 0 && opts[0] != nil && opts[0].BaseURL != "" {
		baseURL = opts[0].BaseURL
	}
	return &SearchBigCartel{URL: URL, baseURL: baseURL, client: client}
}
