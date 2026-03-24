package shopify

import (
	"time"

	json "github.com/goccy/go-json"
)

type shopifyProduct struct {
	ID          int64            `json:"id"`
	Title       string           `json:"title"`
	Handle      string           `json:"handle"`
	BodyHTML    string           `json:"body_html"`
	PublishedAt time.Time        `json:"published_at"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Vendor      string           `json:"vendor"`
	ProductType string           `json:"product_type"`
	Tags        []string         `json:"tags"`
	Variants    []shopifyVariant `json:"variants"`
	Images      []shopifyImage   `json:"images"`
	Options     []shopifyOption  `json:"options"`
}

type shopifyVariant struct {
	ID               int64           `json:"id"`
	Title            string          `json:"title"`
	Option1          *string         `json:"option1"`
	Option2          *string         `json:"option2"`
	Option3          *string         `json:"option3"`
	SKU              string          `json:"sku"`
	RequiresShipping bool            `json:"requires_shipping"`
	Taxable          bool            `json:"taxable"`
	FeaturedImage    json.RawMessage `json:"featured_image"`
	Available        bool            `json:"available"`
	Stock            *int            `json:"stock"`
	Price            string          `json:"price"`
	Grams            int             `json:"grams"`
	CompareAtPrice   *string         `json:"compare_at_price"`
	Position         int             `json:"position"`
	ProductID        int64           `json:"product_id"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type shopifyImage struct {
	ID         int64     `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Position   int       `json:"position"`
	UpdatedAt  time.Time `json:"updated_at"`
	ProductID  int64     `json:"product_id"`
	VariantIDs []int64   `json:"variant_ids"`
	Src        string    `json:"src"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
}

type shopifyOption struct {
	Name     string   `json:"name"`
	Position int      `json:"position"`
	Values   []string `json:"values"`
}
