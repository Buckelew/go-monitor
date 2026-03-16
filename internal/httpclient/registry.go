package httpclient

import "sync"

// ProxyRegistry tracks proxy list versions and shared rotators so consumers
// know when to rebuild their HTTP clients. Call Bump when proxies are updated;
// consumers compare their last-seen version via Version.
type ProxyRegistry struct {
	mu       sync.RWMutex
	versions map[int32]int64
	rotators map[int32]*ProxyRotator
}

func NewProxyRegistry() *ProxyRegistry {
	return &ProxyRegistry{
		versions: make(map[int32]int64),
		rotators: make(map[int32]*ProxyRotator),
	}
}

// Bump increments the version for a proxy list, signaling consumers to rebuild.
// Clears the cached rotator so the next GetOrSetRotator creates a fresh one.
func (r *ProxyRegistry) Bump(proxyListID int32) {
	r.mu.Lock()
	r.versions[proxyListID]++
	delete(r.rotators, proxyListID)
	r.mu.Unlock()
}

// Version returns the current version for a proxy list.
func (r *ProxyRegistry) Version(proxyListID int32) int64 {
	r.mu.RLock()
	v := r.versions[proxyListID]
	r.mu.RUnlock()
	return v
}

// GetOrSetRotator returns the shared rotator for a proxy list. If none exists,
// stores and returns the provided one. This ensures all tasks sharing a proxy
// list use the same rotator, distributing proxies across tasks.
func (r *ProxyRegistry) GetOrSetRotator(proxyListID int32, rotator *ProxyRotator) *ProxyRotator {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.rotators[proxyListID]; ok {
		return existing
	}
	r.rotators[proxyListID] = rotator
	return rotator
}
