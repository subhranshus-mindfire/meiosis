package v1

import (
	"strings"
	"time"
)

// CapabilityToken grants a principal a narrow, time-bounded permission to
// operate within an intent's declared scope. It is the credential a principal
// presents before a write is admitted (FR-1.3/FR-1.4); Intent.Scope is still
// enforced separately at commit time (FR-3.4).
//
// The token is signed by the issuing principal, not the holder: Principal is
// who the capability is granted to, IssuedBy is who granted it and whose key
// produced Signature. A principal can therefore never mint or widen its own
// authority — it can only be delegated a subset of what an issuer holds
// (FR-1.5).
type CapabilityToken struct {
	ID        string    `json:"id"`
	Principal string    `json:"principal"`
	IssuedBy  string    `json:"issued_by"`
	Intent    string    `json:"intent"`
	Scope     Scope     `json:"scope"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Signature string    `json:"signature"`
}

func (t CapabilityToken) Validate() error {
	if !validContentID(t.ID, "cap_") || !validPrincipalID(t.Principal) || !validPrincipalID(t.IssuedBy) || !validContentID(t.Intent, "int_") || t.IssuedAt.IsZero() || t.ExpiresAt.IsZero() || strings.TrimSpace(t.Signature) == "" {
		return ErrInvalidCapabilityToken
	}
	if !t.ExpiresAt.After(t.IssuedAt) {
		return ErrInvalidCapabilityToken
	}
	if t.Scope.Validate() != nil {
		return ErrInvalidCapabilityToken
	}
	return nil
}

// Expired reports whether the token's validity window has passed as of now.
func (t CapabilityToken) Expired(now time.Time) bool {
	return !now.Before(t.ExpiresAt)
}
