// Package sqlite implements the Meiosis storage contract with SQLite.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/mindfire-test/meiosis/pkg/storage"
	"github.com/zeebo/blake3"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS schema_versions (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS principals (
    id TEXT PRIMARY KEY,
    object BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS intents (
    id TEXT PRIMARY KEY,
    object BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS attempts (
    id TEXT PRIMARY KEY,
    object BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS worlds (
    id TEXT PRIMARY KEY,
    object BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS evidence (
    id TEXT PRIMARY KEY,
    object BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS attestations (
    id TEXT PRIMARY KEY,
    object BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS verdicts (
    id TEXT PRIMARY KEY,
    object BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS blobs (
    id TEXT PRIMARY KEY,
    content BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT OR IGNORE INTO schema_versions(version) VALUES (1);
`

// Store is a SQLite-backed implementation of storage.Store.
type Store struct {
	db *sql.DB
}

type transaction struct {
	tx *sql.Tx
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// Open opens databasePath and initializes the M0 schema. Use ":memory:" for
// an isolated in-memory database, primarily in tests.
func Open(databasePath string) (*Store, error) {
	if strings.TrimSpace(databasePath) == "" {
		return nil, fmt.Errorf("database path must not be empty")
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	store := &Store{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("initialize SQLite schema: %w", err)
	}
	return nil
}

func (s *Store) Put(ctx context.Context, kind storage.Kind, key string, value []byte) error {
	return put(ctx, s.db, kind, key, value)
}

func put(ctx context.Context, executor sqlExecutor, kind storage.Kind, key string, value []byte) error {
	table, err := tableName(kind)
	if err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return storage.ErrInvalidKey
	}
	if value == nil {
		value = []byte{}
	}
	query := fmt.Sprintf("INSERT INTO %s(id, object) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET object=excluded.object", table)
	_, err = executor.ExecContext(ctx, query, key, value)
	if err != nil {
		return fmt.Errorf("put %s %q: %w", kind, key, err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, kind storage.Kind, key string) ([]byte, error) {
	return get(ctx, s.db, kind, key)
}

func get(ctx context.Context, executor sqlExecutor, kind storage.Kind, key string) ([]byte, error) {
	table, err := tableName(kind)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(key) == "" {
		return nil, storage.ErrInvalidKey
	}
	var value []byte
	query := fmt.Sprintf("SELECT object FROM %s WHERE id = ?", table)
	if err := executor.QueryRowContext(ctx, query, key).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("get %s %q: %w", kind, key, err)
	}
	return append([]byte(nil), value...), nil
}

func (s *Store) Delete(ctx context.Context, kind storage.Kind, key string) error {
	return deleteRecord(ctx, s.db, kind, key)
}

func deleteRecord(ctx context.Context, executor sqlExecutor, kind storage.Kind, key string) error {
	table, err := tableName(kind)
	if err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return storage.ErrInvalidKey
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", table)
	result, err := executor.ExecContext(ctx, query, key)
	if err != nil {
		return fmt.Errorf("delete %s %q: %w", kind, key, err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *Store) List(ctx context.Context, kind storage.Kind) (map[string][]byte, error) {
	return list(ctx, s.db, kind)
}

func list(ctx context.Context, executor sqlExecutor, kind storage.Kind) (map[string][]byte, error) {
	table, err := tableName(kind)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT id, object FROM %s ORDER BY id", table)
	rows, err := executor.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", kind, err)
	}
	defer rows.Close()
	result := make(map[string][]byte)
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan %s: %w", kind, err)
		}
		result[key] = append([]byte(nil), value...)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s: %w", kind, err)
	}
	return result, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// PutBlob stores value under its BLAKE3-256 content identifier. Repeated
// writes of identical content are idempotent and do not create new records.
func (s *Store) PutBlob(ctx context.Context, value []byte) (storage.BlobID, error) {
	return putBlob(ctx, s.db, value)
}

func putBlob(ctx context.Context, executor sqlExecutor, value []byte) (storage.BlobID, error) {
	digest := blake3.Sum256(value)
	id := storage.BlobID(hex.EncodeToString(digest[:]))
	_, err := executor.ExecContext(ctx, "INSERT OR IGNORE INTO blobs(id, content) VALUES(?, ?)", string(id), value)
	if err != nil {
		return "", fmt.Errorf("put blob %s: %w", id, err)
	}
	return id, nil
}

// GetBlob retrieves content by its BLAKE3-256 hexadecimal identifier.
func (s *Store) GetBlob(ctx context.Context, id storage.BlobID) ([]byte, error) {
	return getBlob(ctx, s.db, id)
}

func getBlob(ctx context.Context, executor sqlExecutor, id storage.BlobID) ([]byte, error) {
	if !validBlobID(id) {
		return nil, storage.ErrInvalidBlob
	}
	var value []byte
	if err := executor.QueryRowContext(ctx, "SELECT content FROM blobs WHERE id = ?", string(id)).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("get blob %s: %w", id, err)
	}
	return append([]byte(nil), value...), nil
}

// Transaction executes fn in a single SQLite transaction. Any callback error
// or commit failure rolls back the transaction.
func (s *Store) Transaction(ctx context.Context, fn func(storage.Tx) error) error {
	if fn == nil {
		return fmt.Errorf("transaction callback must not be nil")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	wrapped := &transaction{tx: tx}
	if err := fn(wrapped); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (t *transaction) Put(ctx context.Context, kind storage.Kind, key string, value []byte) error {
	return put(ctx, t.tx, kind, key, value)
}

func (t *transaction) Get(ctx context.Context, kind storage.Kind, key string) ([]byte, error) {
	return get(ctx, t.tx, kind, key)
}

func (t *transaction) Delete(ctx context.Context, kind storage.Kind, key string) error {
	return deleteRecord(ctx, t.tx, kind, key)
}

func (t *transaction) List(ctx context.Context, kind storage.Kind) (map[string][]byte, error) {
	return list(ctx, t.tx, kind)
}

func (t *transaction) PutBlob(ctx context.Context, value []byte) (storage.BlobID, error) {
	return putBlob(ctx, t.tx, value)
}

func (t *transaction) GetBlob(ctx context.Context, id storage.BlobID) ([]byte, error) {
	return getBlob(ctx, t.tx, id)
}

func validBlobID(id storage.BlobID) bool {
	if len(id) != 64 || string(id) != strings.ToLower(string(id)) {
		return false
	}
	_, err := hex.DecodeString(string(id))
	return err == nil
}

func tableName(kind storage.Kind) (string, error) {
	switch kind {
	case storage.KindPrincipal, storage.KindIntent, storage.KindAttempt,
		storage.KindWorld, storage.KindEvidence, storage.KindAttestation, storage.KindVerdict:
		return string(kind), nil
	default:
		return "", fmt.Errorf("unsupported storage kind %q", kind)
	}
}

var _ storage.Store = (*Store)(nil)
