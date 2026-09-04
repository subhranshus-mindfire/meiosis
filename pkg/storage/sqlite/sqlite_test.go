package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/mindfire-test/meiosis/pkg/storage"
)

func TestOpenInitializesSchemaIdempotently(t *testing.T) {
	first, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := first.Put(context.Background(), storage.KindIntent, "int_1", []byte(`{"id":"int_1"}`)); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	databasePath := t.TempDir() + "/meiosis.db"
	second, err := Open(databasePath)
	if err != nil {
		t.Fatalf("Open() fresh database error = %v", err)
	}
	defer second.Close()
	if err := second.Put(context.Background(), storage.KindWorld, "world-1", []byte("world")); err != nil {
		t.Fatalf("Put() after initialization error = %v", err)
	}
	third, err := Open(databasePath)
	if err != nil {
		t.Fatalf("Open() existing database error = %v", err)
	}
	defer third.Close()
	got, err := third.Get(context.Background(), storage.KindWorld, "world-1")
	if err != nil {
		t.Fatalf("Get() after repeated initialization error = %v", err)
	}
	if string(got) != "world" {
		t.Fatalf("Get() = %q, want world", got)
	}
}

func TestStoreOperations(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	if _, err := store.Get(ctx, storage.KindPrincipal, "missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
	if err := store.Put(ctx, storage.KindPrincipal, "agent:one", []byte("principal")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if err := store.Put(ctx, storage.KindPrincipal, "agent:two", []byte("second")); err != nil {
		t.Fatalf("Put() second error = %v", err)
	}
	items, err := store.List(ctx, storage.KindPrincipal)
	if err != nil || len(items) != 2 {
		t.Fatalf("List() = %#v, %v; want two records", items, err)
	}
	if err := store.Delete(ctx, storage.KindPrincipal, "agent:one"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get(ctx, storage.KindPrincipal, "agent:one"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Get() after delete error = %v, want ErrNotFound", err)
	}
}
