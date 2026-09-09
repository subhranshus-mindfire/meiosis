// Package storage defines the backend-neutral persistence contract used by
// Meiosis components.
package storage

import (
	"errors"
)

var (
	ErrNotFound    = errors.New("storage record not found")
	ErrInvalidKey  = errors.New("invalid storage key")
	ErrInvalidBlob = errors.New("invalid blob identifier")
)

// Kind identifies one of the immutable M0 record collections.
type Kind string

// BlobID is the lower-case hexadecimal BLAKE3-256 identifier of blob content.
type BlobID string

const (
	KindPrincipal   Kind = "principals"
	KindIntent      Kind = "intents"
	KindAttempt     Kind = "attempts"
	KindWorld       Kind = "worlds"
	KindEvidence    Kind = "evidence"
	KindAttestation Kind = "attestations"
	KindVerdict     Kind = "verdicts"
)
