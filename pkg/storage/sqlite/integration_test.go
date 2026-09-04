package sqlite_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mindfire-test/meiosis/pkg/storage"
	"github.com/mindfire-test/meiosis/pkg/storage/sqlite"
)

func TestStorageIntegrationPersistsObjectsAndBlobs(t *testing.T) {
	store, err := sqlite.Open(t.TempDir() + "/meiosis.db")
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer store.Close()

	var publicStore storage.Store = store
	ctx := context.Background()
	object := []byte(`{"id":"att_1","world":"world-1"}`)
	blob := []byte("pack content")
	if err := publicStore.Put(ctx, storage.KindAttempt, "att_1", object); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	blobID, err := publicStore.PutBlob(ctx, blob)
	if err != nil {
		t.Fatalf("PutBlob() error = %v", err)
	}

	gotObject, err := publicStore.Get(ctx, storage.KindAttempt, "att_1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(gotObject) != string(object) {
		t.Fatalf("Get() = %q, want %q", gotObject, object)
	}
	gotBlob, err := publicStore.GetBlob(ctx, blobID)
	if err != nil {
		t.Fatalf("GetBlob() error = %v", err)
	}
	if string(gotBlob) != string(blob) {
		t.Fatalf("GetBlob() = %q, want %q", gotBlob, blob)
	}
}

func TestStorageIntegrationRollsBackPartialMutationAndRecovers(t *testing.T) {
	store, err := sqlite.Open(t.TempDir() + "/meiosis.db")
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	var rolledBackBlob storage.BlobID
	expectedFailure := errors.New("simulated mutation failure")
	if err := store.Transaction(ctx, func(tx storage.Tx) error {
		if err := tx.Put(ctx, storage.KindAttempt, "att_failed", []byte("attempt")); err != nil {
			return err
		}
		var err error
		rolledBackBlob, err = tx.PutBlob(ctx, []byte("failed pack"))
		if err != nil {
			return err
		}
		return expectedFailure
	}); !errors.Is(err, expectedFailure) {
		t.Fatalf("Transaction() error = %v, want %v", err, expectedFailure)
	}
	if _, err := store.Get(ctx, storage.KindAttempt, "att_failed"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("rolled-back object lookup = %v, want ErrNotFound", err)
	}
	if _, err := store.GetBlob(ctx, rolledBackBlob); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("rolled-back blob lookup = %v, want ErrNotFound", err)
	}

	if err := store.Transaction(ctx, func(tx storage.Tx) error {
		if err := tx.Put(ctx, storage.KindAttempt, "att_recovered", []byte("recovered")); err != nil {
			return err
		}
		_, err := tx.PutBlob(ctx, []byte("recovery pack"))
		return err
	}); err != nil {
		t.Fatalf("recovery Transaction() error = %v", err)
	}
	if _, err := store.Get(ctx, storage.KindAttempt, "att_recovered"); err != nil {
		t.Fatalf("recovered object lookup error = %v", err)
	}
}
