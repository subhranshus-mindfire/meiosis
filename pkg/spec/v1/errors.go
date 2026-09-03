package v1

import "errors"

var (
	ErrInvalidWorldHash   = errors.New("invalid world hash")
	ErrInvalidIdentifier  = errors.New("invalid identifier")
	ErrInvalidIntent      = errors.New("invalid intent")
	ErrInvalidPrincipal   = errors.New("invalid principal")
	ErrInvalidAttempt     = errors.New("invalid attempt")
	ErrInvalidEvidence    = errors.New("invalid evidence")
	ErrInvalidAttestation = errors.New("invalid attestation")
	ErrInvalidVerdict     = errors.New("invalid verdict")
	ErrInvalidKey         = errors.New("invalid Ed25519 key")
	ErrKeyMismatch        = errors.New("Ed25519 public and private keys do not match")
)
