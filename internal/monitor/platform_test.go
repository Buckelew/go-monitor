package monitor

import (
	"testing"
)

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		want     Platform
		wantErr  bool
		errMatch string
	}{
		// Shopify
		{"shopify basic", "https://kith.myshopify.com", Shopify, false, ""},
		{"shopify with path", "https://store.myshopify.com/products.json", Shopify, false, ""},
		{"shopify subdomain", "https://my-store.myshopify.com", Shopify, false, ""},
		{"shopify uppercase", "https://KITH.MYSHOPIFY.COM", Shopify, false, ""},
		{"shopify http", "http://kith.myshopify.com", Shopify, false, ""},

		// Big Cartel
		{"bigcartel basic", "https://testshop.bigcartel.com", Bigcartel, false, ""},
		{"bigcartel with path", "https://shop.bigcartel.com/products", Bigcartel, false, ""},
		{"bigcartel uppercase", "https://SHOP.BIGCARTEL.COM", Bigcartel, false, ""},

		// Squarespace
		{"squarespace basic", "https://store.squarespace.com", Squarespace, false, ""},
		{"squarespace with path", "https://mysite.squarespace.com/commerce", Squarespace, false, ""},

		// Reddit
		{"reddit www", "https://www.reddit.com/r/sneakers", Reddit, false, ""},
		{"reddit no www", "https://reddit.com/r/sneakers", Reddit, false, ""},
		{"reddit old", "https://old.reddit.com/r/sneakers", Reddit, false, ""},
		{"reddit with trailing path", "https://www.reddit.com/r/sneakers/new", Reddit, false, ""},
		{"reddit homepage rejects", "https://www.reddit.com", "", true, "subreddit"},
		{"reddit user page rejects", "https://www.reddit.com/u/someuser", "", true, "subreddit"},
		{"reddit no /r/ rejects", "https://www.reddit.com/search?q=test", "", true, "subreddit"},

		// Unsupported
		{"custom domain", "https://kith.com", "", true, "could not detect"},
		{"random site", "https://google.com", "", true, "could not detect"},
		{"empty string", "", "", true, "could not detect"},
		{"just a word", "notaurl", "", true, "could not detect"},

		// Edge cases
		{"myshopify in path not host", "https://evil.com/myshopify.com", "", true, "could not detect"},
		{"bigcartel in path not host", "https://evil.com/bigcartel.com", "", true, "could not detect"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectPlatform(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errMatch)
				}
				if tt.errMatch != "" && !contains(err.Error(), tt.errMatch) {
					t.Fatalf("expected error containing %q, got %q", tt.errMatch, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("DetectPlatform(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
