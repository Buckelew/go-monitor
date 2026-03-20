package httpclient

import (
	"fmt"
	"log"
	"net/url"
	"sync"

	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type Client struct {
	inner        tls_client.HttpClient
	rotator      *ProxyRotator
	currentProxy string // last proxy URL set by RotateProxy
}

// newTLSClient creates a fresh tls_client configured for HTTP/1.1 only.
// HTTP/2 is disabled to prevent readLoop goroutine leaks: SetProxy internally
// replaces the transport but orphans old HTTP/2 connections whose readLoop
// goroutines keep them alive indefinitely.
func newTLSClient(proxyURL string) (tls_client.HttpClient, error) {
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_144),
		tls_client.WithForceHttp1(),
	}
	if proxyURL != "" {
		options = append(options, tls_client.WithProxyUrl(proxyURL))
	}
	return tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
}

func New(rotator *ProxyRotator) (*Client, error) {
	var proxyURL string
	if rotator != nil {
		proxyURL = rotator.Next()
	}
	inner, err := newTLSClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create tls client: %w", err)
	}
	return &Client{inner: inner, rotator: rotator, currentProxy: proxyURL}, nil
}

// RotateProxy creates a fresh tls_client with the next proxy, replacing the
// old one. This avoids the connection leak caused by SetProxy orphaning the
// previous transport's connections.
func (c *Client) RotateProxy() error {
	if c.rotator == nil {
		return nil
	}
	proxy := c.rotator.Next()
	if proxy == "" {
		return nil
	}

	inner, err := newTLSClient(proxy)
	if err != nil {
		return err
	}

	// Best-effort cleanup of old client's connections before swapping.
	c.inner.CloseIdleConnections()
	c.inner = inner
	c.currentProxy = proxy
	return nil
}

// CurrentProxyRaw returns the current proxy in host:port:user:pass format
// suitable for external APIs like the Shape solver.
func (c *Client) CurrentProxyRaw() string {
	if c.currentProxy == "" {
		return ""
	}
	return ProxyURLToRaw(c.currentProxy)
}

// SetRotator swaps the proxy rotator and recreates the tls_client with the
// first proxy from the new rotator.
func (c *Client) SetRotator(rotator *ProxyRotator) {
	c.rotator = rotator
	if rotator != nil {
		if proxy := rotator.Next(); proxy != "" {
			inner, err := newTLSClient(proxy)
			if err != nil {
				log.Printf("httpclient: failed to recreate client on rotator swap: %v", err)
				return
			}
			c.inner.CloseIdleConnections()
			c.inner = inner
		}
	}
}

// Inner returns the underlying tls-client for direct use.
func (c *Client) Inner() tls_client.HttpClient {
	return c.inner
}

// ProxyURLToRaw converts a proxy URL (http://user:pass@host:port) to
// the raw format (host:port:user:pass) expected by external APIs.
func ProxyURLToRaw(proxyURL string) string {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return proxyURL
	}
	host := u.Hostname()
	port := u.Port()
	if u.User == nil {
		return host + ":" + port
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	return fmt.Sprintf("%s:%s:%s:%s", host, port, user, pass)
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
