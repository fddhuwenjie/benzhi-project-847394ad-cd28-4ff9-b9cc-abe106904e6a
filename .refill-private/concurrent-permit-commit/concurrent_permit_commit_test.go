package concurrentpermitcommit_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
	"confinedpermit/internal/storage/jsonstore"
)

type mutationResult struct {
	bundle *domain.PermitBundle
	err    error
}

func TestConcurrentPermitMutationsPreserveBothUpdates(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "permits.json")
	store, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, time.August, 25, 10, 0, 0, 0, time.UTC)
	createPermit(t, store, "permit-a", now)
	createPermit(t, store, "permit-b", now)

	aEntered := make(chan struct{})
	bEntered := make(chan struct{})
	releaseA := make(chan struct{})
	releaseB := make(chan struct{})
	aDone := make(chan mutationResult, 1)
	bDone := make(chan mutationResult, 1)

	go mutatePermit(store, "permit-a", "SPACE-A-UPDATED", "mutate-a", aEntered, releaseA, aDone)
	<-aEntered
	go mutatePermit(store, "permit-b", "SPACE-B-UPDATED", "mutate-b", bEntered, releaseB, bDone)
	<-bEntered

	close(releaseA)
	assertMutationSucceeded(t, <-aDone)
	close(releaseB)
	assertMutationSucceeded(t, <-bDone)

	reopened, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := reopened.Get(ctx, "permit-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := reopened.Get(ctx, "permit-b")
	if err != nil {
		t.Fatal(err)
	}
	if a.Permit.SpaceID != "SPACE-A-UPDATED" || a.Permit.Revision != 2 {
		t.Fatalf("permit-a 的成功更新被并发提交覆盖: space_id=%s revision=%d", a.Permit.SpaceID, a.Permit.Revision)
	}
	if b.Permit.SpaceID != "SPACE-B-UPDATED" || b.Permit.Revision != 2 {
		t.Fatalf("permit-b 更新未持久化: space_id=%s revision=%d", b.Permit.SpaceID, b.Permit.Revision)
	}
}

func createPermit(t *testing.T, store *jsonstore.Store, id string, now time.Time) {
	t.Helper()
	bundle := &domain.PermitBundle{Permit: domain.NewPermit(id, domain.DraftData{
		SpaceID: id, PlannedStart: now, PlannedEnd: now.Add(time.Hour),
	}, now)}
	key := application.IdempotencyKey{Scope: id, Action: "create", RequestID: "create-" + id, Digest: "digest-" + id}
	if _, _, err := store.Create(context.Background(), bundle, key); err != nil {
		t.Fatal(err)
	}
}

func mutatePermit(store *jsonstore.Store, id, spaceID, requestID string, entered chan<- struct{}, release <-chan struct{}, done chan<- mutationResult) {
	key := application.IdempotencyKey{Scope: id, Action: "test-mutate", RequestID: requestID, Digest: "digest-" + requestID}
	bundle, _, err := store.Mutate(context.Background(), id, 1, key, func(work *domain.PermitBundle) error {
		close(entered)
		<-release
		work.Permit.SpaceID = spaceID
		return nil
	})
	done <- mutationResult{bundle: bundle, err: err}
}

func assertMutationSucceeded(t *testing.T, result mutationResult) {
	t.Helper()
	if result.err != nil {
		t.Fatalf("mutation 返回错误: %v", result.err)
	}
	if result.bundle == nil || result.bundle.Permit.Revision != 2 {
		t.Fatalf("mutation 返回异常结果: %#v", result.bundle)
	}
}
