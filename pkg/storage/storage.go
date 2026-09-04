// Package storage defines the backend-neutral persistence contract used by
// Meiosis components.
package storage

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("storage record not found")
	ErrInvalidKey = errors.New("invalid storage key")
)

// Kind identifies one of the immutable M0 record collections.
type Kind string

const (
	KindPrincipal   Kind = "principals"
	KindIntent      Kind = "intents"
	KindAttempt     Kind = "attempts"
	KindWorld       Kind = "worlds"
	KindEvidence    Kind = "evidence"
	KindAttestation Kind = "attestations"
	KindVerdict     Kind = "verdicts"
)

// Store persists serialized Meiosis records without exposing backend details
// to callers. Implementations must copy data they retain or return.
type Store interface {
	Put(ctx context.Context, kind Kind, key string, value []byte) error
	Get(ctx context.Context, kind Kind, key string) ([]byte, error)
	Delete(ctx context.Context, kind Kind, key string) error
	List(ctx context.Context, kind Kind) (map[string][]byte, error)
	Close() error
}
