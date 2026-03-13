package squarespace

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/buckelew/go-monitor/internal/monitor"
)

func productsURL(baseURL string, offset int) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if offset > 0 {
		return fmt.Sprintf("%s?format=json&offset=%d", baseURL, offset)
	}
	return fmt.Sprintf("%s?format=json", baseURL)
}

func parseItems(data []byte, baseURL string) ([]monitor.Item, error) {
	var response squarespaceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}

	var items []monitor.Item
	for _, raw := range response.Items {
		var si squarespaceItem
		if err := json.Unmarshal(raw, &si); err != nil {
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

		items = append(items, monitor.Item{
			URL:      itemURL,
			Title:    si.Title,
			InStock:  inStock,
			ImageURL: si.AssetURL,
			Data:     raw,
		})
	}

	return items, nil
}

func hasNextPage(data []byte) (bool, int) {
	var response squarespaceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return false, 0
	}
	if response.Pagination != nil && response.Pagination.NextPage {
		return true, response.Pagination.NextPageOffset
	}
	return false, 0
}
