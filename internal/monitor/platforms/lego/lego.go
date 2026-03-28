package lego

import (
	"bytes"
	"fmt"
	"io"

	json "github.com/goccy/go-json"

	"github.com/buckelew/go-monitor/internal/monitor"
)

const (
	productsURL = "https://www.lego.com/api/graphql/ProductsQuery"
	productURL  = "https://www.lego.com/api/graphql/ProductQuery"
	maxPerPage  = 500
)

// variantFields is the shared inline fragment for SingleVariantProduct / MultiVariantProduct.
const variantFields = `
      ... on SingleVariantProduct {
        name productCode slug primaryImage
        variant {
          id
          price { centAmount formattedAmount currencyCode }
          attributes { availabilityStatus canAddToBag maxOrderQuantity }
        }
      }
      ... on MultiVariantProduct {
        name productCode slug primaryImage
        variants {
          id
          price { centAmount formattedAmount currencyCode }
          attributes { availabilityStatus canAddToBag maxOrderQuantity }
        }
      }`

// catalogQuery fetches paginated product listings (used for full catalog or search).
const catalogQuery = `query ProductsQuery($query: String!, $page: Int, $perPage: Int) {
  products(query: $query, page: $page, perPage: $perPage) {
    total
    results {
      __typename` + variantFields + `
    }
  }
}`

// singleProductQuery fetches one product by slug (used for fast single-product monitoring).
const singleProductQuery = `query ProductQuery($slug: String!) {
  product(slug: $slug) {
    __typename` + variantFields + `
  }
}`

// --- Request builders ---

type graphqlRequest struct {
	OperationName string         `json:"operationName"`
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables"`
}

func buildCatalogRequest(query string, page int) (io.Reader, error) {
	req := graphqlRequest{
		OperationName: "ProductsQuery",
		Query:         catalogQuery,
		Variables: map[string]any{
			"query":   query,
			"page":    page,
			"perPage": maxPerPage,
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func buildSingleProductRequest(productCode string) (io.Reader, error) {
	req := graphqlRequest{
		OperationName: "ProductQuery",
		Query:         singleProductQuery,
		Variables:     map[string]any{"slug": "-" + productCode},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

// --- Response types ---

type catalogResponse struct {
	Data struct {
		Products struct {
			Total   int             `json:"total"`
			Results []productResult `json:"results"`
		} `json:"products"`
	} `json:"data"`
	Errors []graphqlError `json:"errors"`
}

type singleProductResponse struct {
	Data struct {
		Product *productResult `json:"product"`
	} `json:"data"`
	Errors []graphqlError `json:"errors"`
}

type graphqlError struct {
	Message string `json:"message"`
}

type productResult struct {
	TypeName     string    `json:"__typename"`
	Name         string    `json:"name"`
	ProductCode  string    `json:"productCode"`
	Slug         string    `json:"slug"`
	PrimaryImage string   `json:"primaryImage"`
	Variant      *variant  `json:"variant"`
	Variants     []variant `json:"variants"`
}

type variant struct {
	ID         string     `json:"id"`
	Price      price      `json:"price"`
	Attributes attributes `json:"attributes"`
}

type price struct {
	CentAmount      int    `json:"centAmount"`
	FormattedAmount string `json:"formattedAmount"`
	CurrencyCode    string `json:"currencyCode"`
}

type attributes struct {
	AvailabilityStatus string `json:"availabilityStatus"`
	CanAddToBag        bool   `json:"canAddToBag"`
	MaxOrderQuantity   int    `json:"maxOrderQuantity"`
}

// slimProduct is stored in Item.Data for downstream consumers (embeds, etc.).
type slimProduct struct {
	ProductCode        string `json:"productCode"`
	Slug               string `json:"slug"`
	Price              string `json:"price,omitempty"`
	CentAmount         int    `json:"centAmount,omitempty"`
	AvailabilityStatus string `json:"availabilityStatus,omitempty"`
	CanAddToBag        bool   `json:"canAddToBag"`
}

// --- Parsers ---

func parseCatalogResponse(r io.Reader) (*catalogResponse, error) {
	var resp catalogResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
	}
	return &resp, nil
}

func parseSingleProductResponse(r io.Reader) (*singleProductResponse, error) {
	var resp singleProductResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
	}
	return &resp, nil
}

func resultsToItems(results []productResult) ([]monitor.Item, error) {
	var items []monitor.Item
	for _, p := range results {
		var v *variant
		if p.Variant != nil {
			v = p.Variant
		} else if len(p.Variants) > 0 {
			v = &p.Variants[0]
		}

		inStock := false
		slim := slimProduct{
			ProductCode: p.ProductCode,
			Slug:        p.Slug,
		}

		if v != nil {
			inStock = v.Attributes.CanAddToBag
			slim.Price = v.Price.FormattedAmount
			slim.CentAmount = v.Price.CentAmount
			slim.AvailabilityStatus = v.Attributes.AvailabilityStatus
			slim.CanAddToBag = v.Attributes.CanAddToBag
		}

		data, err := json.Marshal(slim)
		if err != nil {
			return nil, err
		}

		items = append(items, monitor.Item{
			URL:      fmt.Sprintf("https://www.lego.com/en-us/product/%s", p.Slug),
			Title:    p.Name,
			InStock:  inStock,
			ImageURL: p.PrimaryImage,
			Data:     data,
		})
	}
	return items, nil
}
