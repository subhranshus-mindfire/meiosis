package identity

import "sync"

// MemoryRevocationList is an in-process RevocationChecker.
type MemoryRevocationList struct {
	mu      sync.RWMutex
	revoked map[string]struct{}
}

// NewMemoryRevocationList returns an empty revocation list.
func NewMemoryRevocationList() *MemoryRevocationList {
	return &MemoryRevocationList{revoked: make(map[string]struct{})}
}

// Revoke marks tokenID as revoked. Idempotent.
func (l *MemoryRevocationList) Revoke(tokenID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.revoked[tokenID] = struct{}{}
}

// IsRevoked reports whether tokenID has been revoked.
func (l *MemoryRevocationList) IsRevoked(tokenID string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.revoked[tokenID]
	return ok
}

var _ RevocationChecker = (*MemoryRevocationList)(nil)
