package httpclient

import (
	"fmt"
	"sync"

	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type Client struct {
	inner   tls_client.HttpClient
	rotator *ProxyRotator
}

func New(rotator *ProxyRotator) (*Client, error) {
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_144),
	}

	inner, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create tls client: %w", err)
	}

	c := &Client{inner: inner, rotator: rotator}

	// Set initial proxy if available
	if rotator != nil {
		if proxy := rotator.Next(); proxy != "" {
			inner.SetProxy(proxy)
		}
	}

	return c, nil
}

// RotateProxy switches to the next proxy in the rotation.
// Call this before each request for round-robin behavior.
func (c *Client) RotateProxy() error {
	if c.rotator == nil {
		return nil
	}
	proxy := c.rotator.Next()
	if proxy == "" {
		return nil
	}
	return c.inner.SetProxy(proxy)
}

// Inner returns the underlying tls-client for direct use.
func (c *Client) Inner() tls_client.HttpClient {
	return c.inner
}

// ProxyRotator selects proxies in round-robin order.
type ProxyRotator struct {
	proxies []string
	mu      sync.Mutex
	index   int
}

func NewProxyRotator(proxies []string) *ProxyRotator {
	return &ProxyRotator{proxies: proxies}
}

func (r *ProxyRotator) Next() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.proxies) == 0 {
		return ""
	}

	proxy := r.proxies[r.index]
	r.index = (r.index + 1) % len(r.proxies)
	return proxy
}
