package jsonstore_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
	"confinedpermit/internal/storage/jsonstore"
)

func TestFailedSaveDoesNotPolluteCachedDocument(t *testing.T) {
	root := t.TempDir()
	snapshot := filepath.Join(root, "snapshot.json")
	store, err := jsonstore.Open(snapshot)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// Replace the snapshot's parent directory with a regular file so persistence
	// fails after cloneDocumentWithBundle has built the candidate snapshot.
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove temporary root: %v", err)
	}
	if err := os.WriteFile(root, []byte("blocked"), 0o600); err != nil {
		t.Fatalf("block snapshot parent: %v", err)
	}
	bundle := &domain.PermitBundle{Permit: domain.NewPermit("unsaved-permit", domain.DraftData{}, time.Unix(1, 0))}
	key := application.IdempotencyKey{Scope: "permits", Action: "create", RequestID: "req-save-failure", Digest: "digest"}
	if _, _, err := store.Create(context.Background(), bundle, key); err == nil {
		t.Fatal("expected persistence failure")
	}

	got, err := store.Get(context.Background(), bundle.Permit.ID)
	if err == nil || got != nil {
		t.Fatalf("TestFailedSaveDoesNotPolluteCachedDocument: failed save leaked permit into cache: got=%v err=%v", got, err)
	}
	if be, ok := domain.AsBusiness(err); !ok || be.Code != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND after failed save, got %v", err)
	}
}
