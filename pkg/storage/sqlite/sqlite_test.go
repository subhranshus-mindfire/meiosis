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
	defer func() { _ = second.Close() }()
	if putErr := second.Put(context.Background(), storage.KindWorld, "world-1", []byte("world")); putErr != nil {
		t.Fatalf("Put() after initialization error = %v", putErr)
	}
	third, err := Open(databasePath)
	if err != nil {
		t.Fatalf("Open() existing database error = %v", err)
	}
	defer func() { _ = third.Close() }()
	got, getErr := third.Get(context.Background(), storage.KindWorld, "world-1")
	if getErr != nil {
		t.Fatalf("Get() after repeated initialization error = %v", getErr)
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
	defer func() { _ = store.Close() }()
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

func TestBlobPersistenceAndDeduplication(t *testing.T) {
	databasePath := t.TempDir() + "/meiosis.db"
	store, err := Open(databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	ctx := context.Background()
	content := []byte("content-addressed blob")
	first, err := store.PutBlob(ctx, content)
	if err != nil {
		t.Fatalf("PutBlob() error = %v", err)
	}
	second, err := store.PutBlob(ctx, content)
	if err != nil {
		t.Fatalf("PutBlob() duplicate error = %v", err)
	}
	if first != second {
		t.Fatalf("duplicate content IDs differ: %q != %q", first, second)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(databasePath)
	if err != nil {
		t.Fatalf("Open() existing database error = %v", err)
	}
	defer func() { _ = reopened.Close() }()
	got, blobErr := reopened.GetBlob(ctx, first)
	if blobErr != nil {
		t.Fatalf("GetBlob() error = %v", blobErr)
	}
	if string(got) != string(content) {
		t.Fatalf("GetBlob() = %q, want %q", got, content)
	}
	if _, err := reopened.GetBlob(ctx, storage.BlobID("not-a-digest")); !errors.Is(err, storage.ErrInvalidBlob) {
		t.Fatalf("GetBlob() invalid ID error = %v, want ErrInvalidBlob", err)
	}
}

func TestTransactionCommitsAllChanges(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()

	if txErr := store.Transaction(ctx, func(tx storage.Tx) error {
		if putErr := tx.Put(ctx, storage.KindIntent, "int_1", []byte("intent")); putErr != nil {
			return putErr
		}
		_, blobErr := tx.PutBlob(ctx, []byte("blob"))
		return blobErr
	}); txErr != nil {
		t.Fatalf("Transaction() error = %v", txErr)
	}
	if _, err := store.Get(ctx, storage.KindIntent, "int_1"); err != nil {
		t.Fatalf("committed object unavailable: %v", err)
	}
}

func TestTransactionRollsBackAllChanges(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	expected := errors.New("mutation failed")

	if txErr := store.Transaction(ctx, func(tx storage.Tx) error {
		if putErr := tx.Put(ctx, storage.KindAttempt, "att_1", []byte("attempt")); putErr != nil {
			return putErr
		}
		if _, blobErr := tx.PutBlob(ctx, []byte("blob")); blobErr != nil {
			return blobErr
		}
		return expected
	}); !errors.Is(txErr, expected) {
		t.Fatalf("Transaction() error = %v, want %v", txErr, expected)
	}
	if _, err := store.Get(ctx, storage.KindAttempt, "att_1"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("rolled-back object lookup = %v, want ErrNotFound", err)
	}
	if _, err := store.GetBlob(ctx, storage.BlobID("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("rolled-back blob lookup = %v, want ErrNotFound", err)
	}
}
