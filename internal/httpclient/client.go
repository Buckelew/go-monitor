package httpclient

import (
	"fmt"
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

var clientOptions = []tls_client.HttpClientOption{
	tls_client.WithTimeoutSeconds(30),
	tls_client.WithClientProfile(profiles.Chrome_144),
	tls_client.WithForceHttp1(),
}

func newInner(proxy string) (tls_client.HttpClient, error) {
	inner, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create tls client: %w", err)
	}
	if proxy != "" {
		inner.SetProxy(proxy)
	}
	return inner, nil
}

func New(rotator *ProxyRotator) (*Client, error) {
	var proxy string
	if rotator != nil {
		proxy = rotator.Next()
	}
	inner, err := newInner(proxy)
	if err != nil {
		return nil, err
	}
	return &Client{inner: inner, rotator: rotator, currentProxy: proxy}, nil
}

// RotateProxy switches to the next proxy. If the proxy hasn't changed,
// this is a no-op (avoids creating a new transport). When the proxy
// changes, we create a fresh tls_client so the old transport's connections
// can be GC'd — tls_client.SetProxy() internally creates a new transport
// but orphans the old one's connection goroutines.
func (c *Client) RotateProxy() error {
	if c.rotator == nil {
		return nil
	}
	proxy := c.rotator.Next()
	if proxy == "" || proxy == c.currentProxy {
		return nil
	}
	inner, err := newInner(proxy)
	if err != nil {
		return err
	}
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

// SetRotator swaps the proxy rotator and immediately sets the first proxy.
func (c *Client) SetRotator(rotator *ProxyRotator) {
	c.rotator = rotator
	if rotator != nil {
		if proxy := rotator.Next(); proxy != "" {
			inner, err := newInner(proxy)
			if err != nil {
				return
			}
			c.inner.CloseIdleConnections()
			c.inner = inner
			c.currentProxy = proxy
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
