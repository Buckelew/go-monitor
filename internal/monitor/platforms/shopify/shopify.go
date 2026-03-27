package shopify

import (
	"fmt"
	"io"

	json "github.com/goccy/go-json"

	"github.com/buckelew/go-monitor/internal/monitor"
)

func productsURL(baseURL string, page int) string {
	return fmt.Sprintf("%s/products.json?limit=250&page=%d", baseURL, page)
}

// slimProduct contains only the fields downstream consumers need from Item.Data:
// title, images (for notifications), variants with price/available (for embeds + stock).
// This is ~1-2KB vs ~15KB for the full Shopify product JSON.
type slimProduct struct {
	Title    string        `json:"title"`
	Handle   string        `json:"handle"`
	Vendor   string        `json:"vendor,omitempty"`
	Variants []slimVariant `json:"variants"`
	Images   []slimImage   `json:"images"`
}

type slimVariant struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Available bool   `json:"available"`
	Price     string `json:"price"`
}

type slimImage struct {
	Src string `json:"src"`
}

// parseItems stream-parses the Shopify products JSON from r. Each product is
// decoded once into shopifyProduct, then marshalled into a slim JSON with only
// the fields needed downstream — cutting per-item allocation from ~15KB to ~1-2KB
// and eliminating the previous double-parse (RawMessage decode + Unmarshal).
func parseItems(r io.Reader, baseURL string) ([]monitor.Item, error) {
	dec := json.NewDecoder(r)

	// Expect opening {
	if t, err := dec.Token(); err != nil {
		return nil, err
	} else if delim, ok := t.(json.Delim); !ok || delim != '{' {
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
		} else if delim, ok := t.(json.Delim); !ok || delim != '[' {
			return nil, fmt.Errorf("expected [, got %v", t)
		}

		for dec.More() {
			var p shopifyProduct
			if err := dec.Decode(&p); err != nil {
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
			slim := slimProduct{
				Title:  p.Title,
				Handle: p.Handle,
				Vendor: p.Vendor,
			}
			for _, v := range p.Variants {
				slim.Variants = append(slim.Variants, slimVariant{
					ID:        v.ID,
					Title:     v.Title,
					Available: v.Available,
					Price:     v.Price,
				})
			}
			for _, img := range p.Images {
				slim.Images = append(slim.Images, slimImage{Src: img.Src})
				if imageURL == "" {
					imageURL = img.Src
				}
			}

			data, err := json.Marshal(slim)
			if err != nil {
				return nil, err
			}

			items = append(items, monitor.Item{
				URL:      fmt.Sprintf("%s/products/%s", baseURL, monitor.SanitizeSlug(p.Handle)),
				Title:    p.Title,
				InStock:  inStock,
				ImageURL: imageURL,
				Data:     data,
			})
		}

		// Consume closing ]
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
	}

	return items, nil
}
