package shared_snapshot_lost_update_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
	"confinedpermit/internal/storage/jsonstore"
)

func TestSharedSnapshotWritersPreserveBothPermits(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "snapshot.json")
	firstStore, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	secondStore, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	firstMayWrite := make(chan struct{})
	firstDone := make(chan struct{})
	errCh := make(chan error, 2)
	create := func(store *jsonstore.Store, id string) error {
		now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
		permit := domain.NewPermit(id, domain.DraftData{SpaceID: "space-1"}, now)
		key := application.IdempotencyKey{Scope: "permits", Action: "create", RequestID: "create-" + id, Digest: "digest-" + id}
		_, _, err := store.Create(ctx, &domain.PermitBundle{Permit: permit}, key)
		return err
	}
	go func() {
		<-firstMayWrite
		errCh <- create(firstStore, "permit-first")
		close(firstDone)
	}()
	go func() {
		<-firstDone
		errCh <- create(secondStore, "permit-second")
	}()
	close(firstMayWrite)
	for range 2 {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
	reopened, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	items, err := reopened.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.Permit.ID] = true
	}
	if !seen["permit-first"] || !seen["permit-second"] {
		t.Fatalf("共享快照的受控并发写入丢失许可: %#v", seen)
	}
}
