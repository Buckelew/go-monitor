package lego

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCatalogResponse(t *testing.T) {
	body := `{
		"data": {
			"products": {
				"total": 1,
				"results": [{
					"__typename": "SingleVariantProduct",
					"name": "Source Dragon of Motion",
					"productCode": "71822",
					"slug": "source-dragon-of-motion-71822",
					"primaryImage": "https://www.lego.com/cdn/cs/set/assets/bltd43102b059973e3a/71822.png",
					"variant": {
						"id": "abc123",
						"price": {"centAmount": 8999, "formattedAmount": "$89.99", "currencyCode": "USD"},
						"attributes": {"availabilityStatus": "E_AVAILABLE", "canAddToBag": true, "maxOrderQuantity": 5}
					}
				}]
			}
		}
	}`

	resp, err := parseCatalogResponse(strings.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Data.Products.Total)
	require.Len(t, resp.Data.Products.Results, 1)

	p := resp.Data.Products.Results[0]
	assert.Equal(t, "Source Dragon of Motion", p.Name)
	assert.Equal(t, "71822", p.ProductCode)
	require.NotNil(t, p.Variant)
	assert.Equal(t, 8999, p.Variant.Price.CentAmount)
	assert.Equal(t, "$89.99", p.Variant.Price.FormattedAmount)
	assert.Equal(t, "E_AVAILABLE", p.Variant.Attributes.AvailabilityStatus)
	assert.True(t, p.Variant.Attributes.CanAddToBag)
}

func TestParseSingleProductResponse(t *testing.T) {
	body := `{
		"data": {
			"product": {
				"__typename": "SingleVariantProduct",
				"name": "Source Dragon of Motion",
				"productCode": "71822",
				"slug": "source-dragon-of-motion-71822",
				"primaryImage": "https://www.lego.com/cdn/71822.png",
				"variant": {
					"id": "abc123",
					"price": {"centAmount": 8999, "formattedAmount": "$89.99", "currencyCode": "USD"},
					"attributes": {"availabilityStatus": "K_SOLD_OUT", "canAddToBag": false, "maxOrderQuantity": 0}
				}
			}
		}
	}`

	resp, err := parseSingleProductResponse(strings.NewReader(body))
	require.NoError(t, err)
	require.NotNil(t, resp.Data.Product)

	items, err := resultsToItems([]productResult{*resp.Data.Product})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.False(t, items[0].InStock)
	assert.Equal(t, "Source Dragon of Motion", items[0].Title)
}

func TestParseCatalogResponse_MultiVariant(t *testing.T) {
	body := `{
		"data": {
			"products": {
				"total": 1,
				"results": [{
					"__typename": "MultiVariantProduct",
					"name": "LEGO T-Shirt",
					"productCode": "99999",
					"slug": "lego-t-shirt-99999",
					"primaryImage": "https://www.lego.com/img.png",
					"variants": [
						{"id": "v1", "price": {"centAmount": 2499, "formattedAmount": "$24.99", "currencyCode": "USD"}, "attributes": {"availabilityStatus": "E_AVAILABLE", "canAddToBag": true, "maxOrderQuantity": 10}},
						{"id": "v2", "price": {"centAmount": 2499, "formattedAmount": "$24.99", "currencyCode": "USD"}, "attributes": {"availabilityStatus": "K_SOLD_OUT", "canAddToBag": false, "maxOrderQuantity": 0}}
					]
				}]
			}
		}
	}`

	resp, err := parseCatalogResponse(strings.NewReader(body))
	require.NoError(t, err)

	items, err := resultsToItems(resp.Data.Products.Results)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.True(t, items[0].InStock)
	assert.Equal(t, "https://www.lego.com/en-us/product/lego-t-shirt-99999", items[0].URL)
}

func TestResultsToItems_InStockAndOOS(t *testing.T) {
	results := []productResult{
		{
			Name: "Available Set", ProductCode: "11111", Slug: "available-set-11111",
			PrimaryImage: "https://img/1.png",
			Variant: &variant{
				Price:      price{CentAmount: 4999, FormattedAmount: "$49.99"},
				Attributes: attributes{AvailabilityStatus: "E_AVAILABLE", CanAddToBag: true},
			},
		},
		{
			Name: "Sold Out Set", ProductCode: "22222", Slug: "sold-out-set-22222",
			PrimaryImage: "https://img/2.png",
			Variant: &variant{
				Price:      price{CentAmount: 9999, FormattedAmount: "$99.99"},
				Attributes: attributes{AvailabilityStatus: "K_SOLD_OUT", CanAddToBag: false},
			},
		},
	}

	items, err := resultsToItems(results)
	require.NoError(t, err)
	require.Len(t, items, 2)

	assert.True(t, items[0].InStock)
	assert.Contains(t, string(items[0].Data), `"price":"$49.99"`)

	assert.False(t, items[1].InStock)
	assert.Contains(t, string(items[1].Data), `"availabilityStatus":"K_SOLD_OUT"`)
}

func TestParseInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantQuery   string
		wantCode    string
	}{
		{"product URL", "https://www.lego.com/en-us/product/source-dragon-of-motion-71822", "", "71822"},
		{"short product URL", "https://www.lego.com/en-us/product/-71822", "", "71822"},
		{"bare domain", "https://www.lego.com", "", ""},
		{"bare domain trailing slash", "https://www.lego.com/", "", ""},
		{"category URL", "https://www.lego.com/en-us/categories/ninjago", "ninjago", ""},
		{"raw product code", "71822", "", "71822"},
		{"raw search term", "ninjago", "ninjago", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, code := parseInput(tt.input)
			assert.Equal(t, tt.wantQuery, query, "query")
			assert.Equal(t, tt.wantCode, code, "productCode")
		})
	}
}

func TestParseCatalogResponse_GraphQLError(t *testing.T) {
	body := `{"errors":[{"message":"Validation error"}]}`
	_, err := parseCatalogResponse(strings.NewReader(body))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Validation error")
}
