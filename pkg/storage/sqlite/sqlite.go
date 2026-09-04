// Package sqlite implements the Meiosis storage contract with SQLite.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/mindfire-test/meiosis/pkg/storage"
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
INSERT OR IGNORE INTO schema_versions(version) VALUES (1);
`

// Store is a SQLite-backed implementation of storage.Store.
type Store struct {
	db *sql.DB
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
	_, err = s.db.ExecContext(ctx, query, key, value)
	if err != nil {
		return fmt.Errorf("put %s %q: %w", kind, key, err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, kind storage.Kind, key string) ([]byte, error) {
	table, err := tableName(kind)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(key) == "" {
		return nil, storage.ErrInvalidKey
	}
	var value []byte
	query := fmt.Sprintf("SELECT object FROM %s WHERE id = ?", table)
	if err := s.db.QueryRowContext(ctx, query, key).Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("get %s %q: %w", kind, key, err)
	}
	return append([]byte(nil), value...), nil
}

func (s *Store) Delete(ctx context.Context, kind storage.Kind, key string) error {
	table, err := tableName(kind)
	if err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return storage.ErrInvalidKey
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", table)
	result, err := s.db.ExecContext(ctx, query, key)
	if err != nil {
		return fmt.Errorf("delete %s %q: %w", kind, key, err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *Store) List(ctx context.Context, kind storage.Kind) (map[string][]byte, error) {
	table, err := tableName(kind)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT id, object FROM %s ORDER BY id", table)
	rows, err := s.db.QueryContext(ctx, query)
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
