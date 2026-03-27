package bigcartel

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/buckelew/go-monitor/internal/monitor"
)

func productsURL(storeURL, apiBase string) (string, error) {
	u, err := url.Parse(storeURL)
	if err != nil {
		return "", fmt.Errorf("invalid store URL: %w", err)
	}

	// For test overrides, extract subdomain and use the test base.
	if apiBase != "" {
		parts := strings.Split(u.Hostname(), ".")
		subdomain := parts[0]
		return fmt.Sprintf("%s/%s/products.json", strings.TrimRight(apiBase, "/"), subdomain), nil
	}

	// For *.bigcartel.com domains, use the API with the subdomain.
	host := u.Hostname()
	if strings.HasSuffix(host, ".bigcartel.com") {
		subdomain := strings.TrimSuffix(host, ".bigcartel.com")
		return fmt.Sprintf("https://api.bigcartel.com/%s/products.json", subdomain), nil
	}

	// Custom domains serve the products JSON directly.
	return fmt.Sprintf("%s/products.json", strings.TrimRight(storeURL, "/")), nil
}

func parseItems(r io.Reader, storeURL string) ([]monitor.Item, error) {
	var products []bigcartelProduct
	if err := json.NewDecoder(r).Decode(&products); err != nil {
		return nil, err
	}

	var items []monitor.Item
	for _, p := range products {
		inStock := p.Status != "sold_out"
		if inStock && len(p.Options) > 0 {
			// All options sold out means the product is sold out
			allSoldOut := true
			for _, opt := range p.Options {
				if !opt.SoldOut {
					allSoldOut = false
					break
				}
			}
			if allSoldOut {
				inStock = false
			}
		}

		var imageURL string
		if len(p.Images) > 0 {
			imageURL = p.Images[0].SecureURL
		}

		raw, err := json.Marshal(p)
		if err != nil {
			return nil, err
		}

		items = append(items, monitor.Item{
			URL:      fmt.Sprintf("%s/product/%s", strings.TrimRight(storeURL, "/"), monitor.SanitizeSlug(p.Permalink)),
			Title:    p.Name,
			InStock:  inStock,
			ImageURL: imageURL,
			Data:     raw,
		})
	}

	return items, nil
}
