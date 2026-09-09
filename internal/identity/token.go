// Package identity issues and verifies capability tokens: signed,
// time-bounded, path-scoped grants that let a principal act on an intent.
// See SRS §6.1, FR-1.3/FR-1.4.
package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/mindfire-test/meiosis/pkg/crypto"
	specv1 "github.com/mindfire-test/meiosis/pkg/spec/v1"
)

// DefaultTTL is the default capability token lifetime. NFR-4.3 requires
// capability tokens to be short-lived, defaulting to one hour.
const DefaultTTL = time.Hour

// tokenIDEncoding mirrors the lower-case, unpadded base32 alphabet used for
// content IDs elsewhere in spec/v1. Capability token IDs are random rather
// than content-derived: two tokens issued with identical fields still need
// independent identities so one can be revoked without the other.
var tokenIDEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// IssueParams describes the capability being granted.
type IssueParams struct {
	Principal string        // who the token authorizes (the holder)
	Intent    string        // the intent this capability is scoped to
	Scope     specv1.Scope  // path glob constraints; Deny overrides Allow
	IssuedBy  string        // who is granting it (the issuer; signs the token)
	IssuedAt  time.Time     // zero means time.Now().UTC()
	TTL       time.Duration // zero means DefaultTTL
}

// Issue signs a new capability token with the issuer's private key. The
// issuer, not the holder, signs the token, so a principal can never mint or
// widen its own authority — it can only be delegated a subset of what an
// issuer already holds (FR-1.5).
func Issue(params IssueParams, issuerKey ed25519.PrivateKey) (specv1.CapabilityToken, error) {
	issuedAt := params.IssuedAt
	if issuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	}
	ttl := params.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	id, err := newTokenID()
	if err != nil {
		return specv1.CapabilityToken{}, fmt.Errorf("identity: generate token id: %w", err)
	}

	token := specv1.CapabilityToken{
		ID:        id,
		Principal: params.Principal,
		IssuedBy:  params.IssuedBy,
		Intent:    params.Intent,
		Scope:     params.Scope,
		IssuedAt:  issuedAt,
		ExpiresAt: issuedAt.Add(ttl),
	}
	signature, err := crypto.Sign(token, issuerKey)
	if err != nil {
		return specv1.CapabilityToken{}, fmt.Errorf("identity: sign token: %w", err)
	}
	token.Signature = signature

	if err := token.Validate(); err != nil {
		return specv1.CapabilityToken{}, fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	return token, nil
}

// Verify checks a capability token's signature, expiration and revocation
// status against the issuer's public key. It does not check the token's
// scope against any specific path — call Authorize for that at the point a
// write is intercepted.
func Verify(token specv1.CapabilityToken, issuerKey ed25519.PublicKey, revoked RevocationChecker, now time.Time) error {
	if err := token.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrTokenAltered, err)
	}
	ok, err := crypto.Verify(token, token.Signature, issuerKey)
	if err != nil || !ok {
		return ErrTokenAltered
	}
	if token.Expired(now) {
		return ErrTokenExpired
	}
	if revoked != nil && revoked.IsRevoked(token.ID) {
		return ErrTokenRevoked
	}
	return nil
}

// Authorize is the single check a write path — a git push, an MCP tool call,
// a CLI mutation — should run before admitting an operation performed under a
// capability token. It is the "local interception pipeline" integration point
// named in FR-1.3: callers run this once instead of separately re-checking
// signature, expiry, revocation and scope.
//
// The scope check here duplicates the glob evaluation the dedicated scope
// engine (FR-3.4, internal/scope) also performs, against a different source
// of allow/deny rules: this checks the token's own grant, not the intent's
// declared bounds. A real write path is expected to run both.
func Authorize(token specv1.CapabilityToken, issuerKey ed25519.PublicKey, revoked RevocationChecker, path string, now time.Time) error {
	if err := Verify(token, issuerKey, revoked, now); err != nil {
		return err
	}
	allowed, err := scopeAllows(token.Scope, path)
	if err != nil {
		return fmt.Errorf("identity: evaluate scope: %w", err)
	}
	if !allowed {
		return fmt.Errorf("%w: %s", ErrScopeDenied, path)
	}
	return nil
}

func scopeAllows(scope specv1.Scope, path string) (bool, error) {
	for _, pattern := range scope.Deny {
		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			return false, err
		}
		if matched {
			return false, nil
		}
	}
	for _, pattern := range scope.Allow {
		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

// Encode returns a capability token's canonical JSON encoding: the
// deterministic form used for storage and transmission.
func Encode(token specv1.CapabilityToken) ([]byte, error) {
	return specv1.Canonicalize(token)
}

// Decode parses a capability token from its JSON encoding.
func Decode(data []byte) (specv1.CapabilityToken, error) {
	var token specv1.CapabilityToken
	if err := json.Unmarshal(data, &token); err != nil {
		return specv1.CapabilityToken{}, fmt.Errorf("identity: decode token: %w", err)
	}
	return token, nil
}

func newTokenID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "cap_" + tokenIDEncoding.EncodeToString(buf), nil
}
