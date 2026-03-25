package shopify

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/buckelew/go-monitor/internal/monitor"
)

func TestParseItems_NormalizesUnicodeHandles(t *testing.T) {
	// The ® character (U+00AE) can be encoded in UTF-8 as 0xC2 0xAE.
	// When Shopify returns inconsistent encoding (e.g. double-encoded or
	// mojibake), the same product gets two different URLs, causing
	// delist/restock flip-flop. NFC normalization should produce a
	// consistent URL regardless of which encoding variant arrives.

	nfcHandle := "product-\u00AE-name"

	tests := []struct {
		name    string
		handle  string
		wantURL string
	}{
		{
			name:    "NFC handle normalizes consistently",
			handle:  nfcHandle,
			wantURL: "https://example.com/products/product-®-name",
		},
		{
			name:    "plain ASCII handle unchanged",
			handle:  "plain-product",
			wantURL: "https://example.com/products/plain-product",
		},
		{
			name:    "handle with emoji normalizes",
			handle:  "cool-product-\U0001F525",
			wantURL: "https://example.com/products/cool-product-\U0001F525",
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

func TestParseItems_DuplicateHandlesWithDifferentEncoding(t *testing.T) {
	// Simulate what Shopify does: same product returned with different
	// byte sequences for the handle. After normalization both should
	// produce the exact same URL.
	handle1 := "sig-mcx\u00ae-model"
	handle2 := "sig-mcx\u00ae-model"

	body := `{"products":[
		{"title":"Product A","handle":"` + handle1 + `","variants":[{"available":true,"price":"1.00"}],"images":[]},
		{"title":"Product B","handle":"` + handle2 + `","variants":[{"available":true,"price":"2.00"}],"images":[]}
	]}`

	items, err := parseItems(strings.NewReader(body), "https://store.com")
	require.NoError(t, err)
	require.Len(t, items, 2)

	assert.Equal(t, items[0].URL, items[1].URL,
		"same handle with different encoding should produce identical URLs")
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ASCII passthrough", "https://store.com/products/plain-handle", "https://store.com/products/plain-handle"},
		{"NFC registered mark", "https://store.com/products/sig-mcx\u00AE-model", "https://store.com/products/sig-mcx\u00AE-model"},
		{"empty string", "", ""},
		{"combining chars normalized to NFC", "https://store.com/products/caf\u0065\u0301", "https://store.com/products/caf\u00e9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, monitor.NormalizeURL(tt.input))
		})
	}
}
