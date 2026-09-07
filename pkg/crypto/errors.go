package crypto

import "errors"

var (
	ErrInvalidKey       = errors.New("invalid Ed25519 key")
	ErrKeyMismatch      = errors.New("Ed25519 public and private keys do not match")
	ErrInvalidSignature = errors.New("invalid Ed25519 signature")
)
