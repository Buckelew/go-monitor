package squarespace

import (
	"fmt"
	"io"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/buckelew/go-monitor/internal/monitor"
)

func productsURL(baseURL string, offset int) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if offset > 0 {
		return fmt.Sprintf("%s?format=json&offset=%d", baseURL, offset)
	}
	return fmt.Sprintf("%s?format=json", baseURL)
}

// parseResult holds items and pagination from a single page fetch.
type parseResult struct {
	items      []monitor.Item
	hasNext    bool
	nextOffset int
}

// parseItems stream-parses the Squarespace JSON from r, extracting both items
// and pagination in a single pass. Each item is decoded once into squarespaceItem
// and marshalled back for Item.Data (no double-parse via RawMessage).
func parseItems(r io.Reader, baseURL string) (*parseResult, error) {
	dec := json.NewDecoder(r)

	// Expect opening {
	if t, err := dec.Token(); err != nil {
		return nil, err
	} else if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("expected {, got %v", t)
	}

	result := &parseResult{}

	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}

		switch key {
		case "items":
			// Stream the items array
			if t, err := dec.Token(); err != nil {
				return nil, err
			} else if delim, ok := t.(json.Delim); !ok || delim != '[' {
				return nil, fmt.Errorf("expected [, got %v", t)
			}

			for dec.More() {
				var si squarespaceItem
				if err := dec.Decode(&si); err != nil {
					return nil, err
				}

				inStock := false
				if si.StructuredContent != nil {
					for _, v := range si.StructuredContent.Variants {
						if v.Unlimited || v.QtyInStock > 0 {
							inStock = true
							break
						}
					}
				}

				itemURL := si.FullURL
				if itemURL == "" {
					itemURL = fmt.Sprintf("%s/%s", strings.TrimRight(baseURL, "/"), si.URLId)
				}

				data, err := json.Marshal(si)
				if err != nil {
					return nil, err
				}

				result.items = append(result.items, monitor.Item{
					URL:      monitor.NormalizeURL(itemURL),
					Title:    si.Title,
					InStock:  inStock,
					ImageURL: si.AssetURL,
					Data:     data,
				})
			}

			// Consume closing ]
			if _, err := dec.Token(); err != nil {
				return nil, err
			}

		case "pagination":
			var pag sqsPagination
			if err := dec.Decode(&pag); err != nil {
				return nil, err
			}
			result.hasNext = pag.NextPage
			result.nextOffset = pag.NextPageOffset

		default:
			// Skip unknown fields
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}
