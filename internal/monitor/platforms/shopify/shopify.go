package shopify

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/buckelew/go-monitor/internal/monitor"
)

func productsURL(baseURL string, requestNum uint64) string {
	cacheBust := strings.Repeat("%20", int(requestNum%100)+1)
	return fmt.Sprintf("%s/collections/all%s/products.json?limit=250", baseURL, cacheBust)
}

func parseItems(data []byte, baseURL string) ([]monitor.Item, error) {
	var response productsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}

	var items []monitor.Item
	for _, raw := range response.Products {
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

	return items, nil
}
