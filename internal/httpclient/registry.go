package httpclient

import "sync"

// ProxyRegistry tracks proxy list versions so consumers know when to rebuild
// their HTTP clients. Call Bump when proxies are updated; consumers compare
// their last-seen version via Version.
type ProxyRegistry struct {
	mu       sync.RWMutex
	versions map[int32]int64
}

func NewProxyRegistry() *ProxyRegistry {
	return &ProxyRegistry{versions: make(map[int32]int64)}
}

// Bump increments the version for a proxy list, signaling consumers to rebuild.
func (r *ProxyRegistry) Bump(proxyListID int32) {
	r.mu.Lock()
	r.versions[proxyListID]++
	r.mu.Unlock()
}

// Version returns the current version for a proxy list.
func (r *ProxyRegistry) Version(proxyListID int32) int64 {
	r.mu.RLock()
	v := r.versions[proxyListID]
	r.mu.RUnlock()
	return v
}
