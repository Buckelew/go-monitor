package shopify

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/buckelew/go-monitor/internal/monitor"
)

func productsURL(baseURL string, page int) string {
	return fmt.Sprintf("%s/products.json?limit=250&page=%d", baseURL, page)
}

// parseItems stream-parses the Shopify products JSON from r without buffering
// the entire response body. It walks the token stream to the "products" array,
// then decodes each product individually, keeping only the per-product
// json.RawMessage (for Item.Data) rather than the full response bytes.
func parseItems(r io.Reader, baseURL string) ([]monitor.Item, error) {
	dec := json.NewDecoder(r)

	// Expect opening {
	if t, err := dec.Token(); err != nil {
		return nil, err
	} else if t != json.Delim('{') {
		return nil, fmt.Errorf("expected {, got %v", t)
	}

	var items []monitor.Item
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}

		if key != "products" {
			// Skip non-products fields
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return nil, err
			}
			continue
		}

		// Read opening bracket of products array
		if t, err := dec.Token(); err != nil {
			return nil, err
		} else if t != json.Delim('[') {
			return nil, fmt.Errorf("expected [, got %v", t)
		}

		for dec.More() {
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				return nil, err
			}

			var p shopifyProduct
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}

			inStock := false
			for _, v := range p.Variants {
				if v.Available {
					inStock = true
					break
				}
			}

			var imageURL string
			if len(p.Images) > 0 {
				imageURL = p.Images[0].Src
			}

			items = append(items, monitor.Item{
				URL:      fmt.Sprintf("%s/products/%s", baseURL, p.Handle),
				Title:    p.Title,
				InStock:  inStock,
				ImageURL: imageURL,
				Data:     raw,
			})
		}

		// Consume closing ]
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
	}

	return items, nil
}
