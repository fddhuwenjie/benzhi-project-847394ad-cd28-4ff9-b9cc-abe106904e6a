package list_result_cache_alias_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
	"confinedpermit/internal/storage/jsonstore"
)

func TestListResultMutationDoesNotPolluteStoreCache(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "permits.json")
	store, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	permit := domain.NewPermit("permit-list-alias", domain.DraftData{
		SpaceID:      "SPACE-ORIGINAL",
		PlannedStart: now.Add(time.Hour),
		PlannedEnd:   now.Add(2 * time.Hour),
	}, now)
	key := application.IdempotencyKey{
		Scope: "permits", Action: "create", RequestID: "create-list-alias", Digest: "digest-list-alias",
	}
	if _, _, err := store.Create(ctx, &domain.PermitBundle{Permit: permit}, key); err != nil {
		t.Fatal(err)
	}

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("期望一个许可，实际为 %d", len(listed))
	}
	listed[0].Permit.SpaceID = "SPACE-CALLER-MUTATED"

	cached, err := store.Get(ctx, permit.ID)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := reopened.Get(ctx, permit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cached.Permit.SpaceID != persisted.Permit.SpaceID {
		t.Fatalf("List 返回值污染缓存：内存 space_id=%q，磁盘 space_id=%q", cached.Permit.SpaceID, persisted.Permit.SpaceID)
	}
}
