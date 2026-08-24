package jsonstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
)

func testBundle(now time.Time) *domain.PermitBundle {
	p := domain.NewPermit("permit-store", domain.DraftData{SpaceID: "SPACE", PlannedStart: now, PlannedEnd: now.Add(time.Hour)}, now)
	return &domain.PermitBundle{Permit: p}
}

func TestStorePersistsRevisionAndIdempotency(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "permits.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	createKey := application.IdempotencyKey{Scope: "permits", Action: "create", RequestID: "req-create", Digest: "digest-a"}
	created, replay, err := store.Create(ctx, testBundle(time.Now().UTC()), createKey)
	if err != nil || replay || created.Permit.Revision != 1 {
		t.Fatalf("创建失败: replay=%v permit=%#v err=%v", replay, created, err)
	}
	replayed, replay, err := store.Create(ctx, testBundle(time.Now().UTC()), createKey)
	if err != nil || !replay || replayed.Permit.ID != created.Permit.ID {
		t.Fatalf("创建幂等重放失败: %v %v", replay, err)
	}
	badKey := createKey
	badKey.Digest = "different"
	if _, _, err := store.Create(ctx, testBundle(time.Now().UTC()), badKey); err == nil {
		t.Fatal("相同 request_id 的不同载荷应被拒绝")
	}
	mutateKey := application.IdempotencyKey{Scope: created.Permit.ID, Action: "submit", RequestID: "req-submit", Digest: "digest-submit"}
	mutated, replay, err := store.Mutate(ctx, created.Permit.ID, 1, mutateKey, func(b *domain.PermitBundle) error { b.Permit.Status = domain.StatusPendingReview; return nil })
	if err != nil || replay || mutated.Permit.Revision != 2 {
		t.Fatalf("更新失败: %#v %v", mutated, err)
	}
	mutated, replay, err = store.Mutate(ctx, created.Permit.ID, 1, mutateKey, func(*domain.PermitBundle) error { t.Fatal("幂等重放不应再次执行变更函数"); return nil })
	if err != nil || !replay || mutated.Permit.Revision != 2 {
		t.Fatalf("更新重放失败: %#v %v", mutated, err)
	}
	conflictKey := application.IdempotencyKey{Scope: created.Permit.ID, Action: "other", RequestID: "req-other", Digest: "digest-other"}
	if _, _, err := store.Mutate(ctx, created.Permit.ID, 1, conflictKey, func(*domain.PermitBundle) error { return nil }); err == nil {
		t.Fatal("过期 revision 应冲突")
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get(ctx, created.Permit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Permit.Revision != 2 || got.Permit.Status != domain.StatusPendingReview {
		t.Fatalf("重启后数据错误: %#v", got.Permit)
	}
}
