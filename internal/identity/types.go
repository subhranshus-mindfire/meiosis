package identity

// RevocationChecker reports whether a capability token has been revoked.
// FR-1.7 requires revocation to propagate within 5s once a durable, shared
// store backs this; MemoryRevocationList is enough for single-process use
// and tests until then.
type RevocationChecker interface {
	IsRevoked(tokenID string) bool
}
