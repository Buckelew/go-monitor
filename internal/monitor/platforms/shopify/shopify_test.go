package shopify

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/buckelew/go-monitor/internal/monitor"
)

func TestParseItems_StripsNonASCIIFromHandles(t *testing.T) {
	tests := []struct {
		name    string
		handle  string
		wantURL string
	}{
		{
			name:    "registered mark stripped",
			handle:  "product-\u00AE-name",
			wantURL: "https://example.com/products/product--name",
		},
		{
			name:    "plain ASCII handle unchanged",
			handle:  "plain-product",
			wantURL: "https://example.com/products/plain-product",
		},
		{
			name:    "trademark stripped",
			handle:  "cool-product-\u2122",
			wantURL: "https://example.com/products/cool-product-",
		},
		{
			name:    "replacement chars stripped",
			handle:  "product-\uFFFD\uFFFD-name",
			wantURL: "https://example.com/products/product--name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"products":[{"title":"Test","handle":"` + tt.handle + `","variants":[{"available":true,"price":"9.99"}],"images":[]}]}`
			items, err := parseItems(strings.NewReader(body), "https://example.com")
			require.NoError(t, err)
			require.Len(t, items, 1)
			assert.Equal(t, tt.wantURL, items[0].URL)
		})
	}
}

func TestParseItems_MojibakeAndCleanHandleProduceSameURL(t *testing.T) {
	// The real bug: Shopify sends ® (0xC2 0xAE) one request, then broken
	// bytes the next that Go decodes as U+FFFD. After sanitization both
	// must produce the same URL.
	cleanHandle := "sig-mcx\u00ae-model"     // has ®
	mojibakeHandle := "sig-mcx\uFFFD\uFFFD-model" // has ��

	body1 := `{"products":[{"title":"P","handle":"` + cleanHandle + `","variants":[{"available":true,"price":"1.00"}],"images":[]}]}`
	body2 := `{"products":[{"title":"P","handle":"` + mojibakeHandle + `","variants":[{"available":true,"price":"1.00"}],"images":[]}]}`

	items1, err := parseItems(strings.NewReader(body1), "https://store.com")
	require.NoError(t, err)
	items2, err := parseItems(strings.NewReader(body2), "https://store.com")
	require.NoError(t, err)

	assert.Equal(t, items1[0].URL, items2[0].URL,
		"clean ® and mojibake handles must produce identical URLs")
}

func TestParseItems_VariantIDInData(t *testing.T) {
	body := `{"products":[{"title":"Test","handle":"test","variants":[{"id":12345,"available":true,"price":"9.99"}],"images":[]}]}`
	items, err := parseItems(strings.NewReader(body), "https://example.com")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Contains(t, string(items[0].Data), `"id":12345`)
}

func TestSanitizeSlug(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ASCII passthrough", "plain-handle", "plain-handle"},
		{"strips registered mark", "sig-mcx\u00AE-model", "sig-mcx-model"},
		{"strips replacement chars", "sig-mcx\uFFFD\uFFFD-model", "sig-mcx-model"},
		{"strips trademark", "product\u2122-name", "product-name"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, monitor.SanitizeSlug(tt.input))
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ASCII passthrough", "https://store.com/products/plain-handle", "https://store.com/products/plain-handle"},
		{"NFC registered mark preserved", "https://store.com/products/sig-mcx\u00AE-model", "https://store.com/products/sig-mcx\u00AE-model"},
		{"empty string", "", ""},
		{"combining chars normalized to NFC", "https://store.com/products/caf\u0065\u0301", "https://store.com/products/caf\u00e9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, monitor.NormalizeURL(tt.input))
		})
	}
}
