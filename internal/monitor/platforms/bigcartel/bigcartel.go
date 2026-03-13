package bigcartel

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/buckelew/go-monitor/internal/monitor"
)

func productsURL(storeURL, apiBase string) (string, error) {
	u, err := url.Parse(storeURL)
	if err != nil {
		return "", fmt.Errorf("invalid store URL: %w", err)
	}

	// Extract subdomain from e.g. "coolshop.bigcartel.com"
	parts := strings.Split(u.Hostname(), ".")
	if len(parts) == 0 {
		return "", fmt.Errorf("could not extract subdomain from %s", storeURL)
	}
	subdomain := parts[0]

	if apiBase != "" {
		return fmt.Sprintf("%s/%s/products.json", strings.TrimRight(apiBase, "/"), subdomain), nil
	}
	return fmt.Sprintf("https://api.bigcartel.com/%s/products.json", subdomain), nil
}

func parseItems(data []byte, storeURL string) ([]monitor.Item, error) {
	var products []bigcartelProduct
	if err := json.Unmarshal(data, &products); err != nil {
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
			URL:      fmt.Sprintf("%s/product/%s", strings.TrimRight(storeURL, "/"), p.Permalink),
			Title:    p.Name,
			InStock:  inStock,
			ImageURL: imageURL,
			Data:     raw,
		})
	}

	return items, nil
}
