package lego

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"

	"github.com/buckelew/go-monitor/internal/httpclient"
	"github.com/buckelew/go-monitor/internal/monitor"
)

type SearchLego struct {
	URL         string
	query       string // resolved search term for catalog mode
	productCode string // set for single-product mode (fast)
	client      *httpclient.Client
}

func (s *SearchLego) Platform() monitor.Platform { return monitor.Lego }
func (s *SearchLego) Client() *httpclient.Client  { return s.client }

func (s *SearchLego) FetchProducts(ctx context.Context) (*monitor.FetchResult, error) {
	if err := s.client.RotateProxy(); err != nil {
		return nil, fmt.Errorf("failed to rotate proxy: %w", err)
	}

	if s.productCode != "" {
		return s.fetchSingleProduct(ctx)
	}
	return s.fetchCatalog(ctx)
}

// fetchSingleProduct queries a single product by code — fast, single request.
func (s *SearchLego) fetchSingleProduct(ctx context.Context) (*monitor.FetchResult, error) {
	body, err := buildSingleProductRequest(s.productCode)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, productURL, body)
	if err != nil {
		return nil, err
	}
	setHeaders(req)

	start := time.Now()
	resp, err := s.client.Inner().Do(req)
	duration := time.Since(start)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, productURL)
	}

	cr := &monitor.CountingReader{R: resp.Body}
	gqlResp, err := parseSingleProductResponse(cr)
	io.Copy(io.Discard, cr)
	if err != nil {
		return nil, err
	}

	if gqlResp.Data.Product == nil {
		return &monitor.FetchResult{
			Meta: monitor.FetchMeta{StatusCode: resp.StatusCode, Duration: duration, BodySize: cr.N, Proxy: s.client.CurrentProxyRaw()},
		}, nil
	}

	items, err := resultsToItems([]productResult{*gqlResp.Data.Product})
	if err != nil {
		return nil, err
	}

	return &monitor.FetchResult{
		Items: items,
		Meta:  monitor.FetchMeta{StatusCode: resp.StatusCode, Duration: duration, BodySize: cr.N, Proxy: s.client.CurrentProxyRaw()},
	}, nil
}

// fetchCatalog paginates through the full catalog or search results.
func (s *SearchLego) fetchCatalog(ctx context.Context) (*monitor.FetchResult, error) {
	var allItems []monitor.Item
	var totalSize int
	var lastStatus int
	start := time.Now()

	for page := 1; ; page++ {
		body, err := buildCatalogRequest(s.query, page)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, productsURL, body)
		if err != nil {
			return nil, err
		}
		setHeaders(req)

		resp, err := s.client.Inner().Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status %d from %s (page %d)", resp.StatusCode, productsURL, page)
		}

		cr := &monitor.CountingReader{R: resp.Body}
		gqlResp, err := parseCatalogResponse(cr)
		io.Copy(io.Discard, cr)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		items, err := resultsToItems(gqlResp.Data.Products.Results)
		if err != nil {
			return nil, err
		}

		allItems = append(allItems, items...)
		totalSize += cr.N
		lastStatus = resp.StatusCode

		if len(allItems) >= gqlResp.Data.Products.Total || len(gqlResp.Data.Products.Results) < maxPerPage {
			break
		}
	}

	return &monitor.FetchResult{
		Items: allItems,
		Meta:  monitor.FetchMeta{StatusCode: lastStatus, Duration: time.Since(start), BodySize: totalSize, Proxy: s.client.CurrentProxyRaw()},
	}, nil
}

func setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-locale", "en-US")
}

func NewSearchLego(URL string, client *httpclient.Client) *SearchLego {
	query, productCode := parseInput(URL)
	return &SearchLego{URL: URL, query: query, productCode: productCode, client: client}
}

// parseInput determines the scraper mode from the input URL.
//
// Product URL → single-product mode (fast):
//
//	"https://www.lego.com/en-us/product/-71822" → productCode="71822"
//
// Bare domain → full catalog mode (paginated):
//
//	"https://www.lego.com" → query="" (all products)
//
// Category/search → filtered catalog mode:
//
//	"https://www.lego.com/en-us/categories/ninjago" → query="ninjago"
func parseInput(raw string) (query, productCode string) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		// Raw input: if all digits, treat as product code; otherwise search term.
		if isDigits(raw) {
			return "", raw
		}
		return raw, ""
	}

	path := strings.Trim(u.Path, "/")
	if path == "" {
		return "", "" // bare domain → full catalog
	}

	parts := strings.Split(path, "/")
	slug := parts[len(parts)-1]

	// Product URLs: extract trailing product code.
	if idx := strings.LastIndex(slug, "-"); idx >= 0 {
		code := slug[idx+1:]
		if isDigits(code) {
			return "", code
		}
	}
	if isDigits(slug) {
		return "", slug
	}

	return slug, ""
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
