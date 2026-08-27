package replaycache_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
	"confinedpermit/internal/storage/jsonstore"
)

func TestIdempotencyReplayResultsAreIsolated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	store, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	bundle := &domain.PermitBundle{Permit: &domain.WorkPermit{
		ID: "permit-replay", SpaceID: "SPACE-ORIGINAL", Status: domain.StatusDraft, Revision: 1,
		Workers: []domain.Worker{{ID: "worker-1", Name: "原始作业员"}}, CreatedAt: now, UpdatedAt: now,
	}}
	key := application.IdempotencyKey{Scope: "permits", Action: "create", RequestID: "request-replay", Digest: "stable-digest"}

	if _, replay, err := store.Create(context.Background(), bundle, key); err != nil || replay {
		t.Fatalf("首次创建失败: replay=%v err=%v", replay, err)
	}
	firstReplay, replay, err := store.Create(context.Background(), bundle, key)
	if err != nil || !replay {
		t.Fatalf("首次幂等重放失败: replay=%v err=%v", replay, err)
	}
	firstReplay.Permit.SpaceID = "SPACE-POLLUTED"
	firstReplay.Permit.Workers[0].Name = "被调用方修改"

	secondReplay, replay, err := store.Create(context.Background(), bundle, key)
	if err != nil || !replay {
		t.Fatalf("第二次幂等重放失败: replay=%v err=%v", replay, err)
	}
	reopened, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	durableReplay, replay, err := reopened.Create(context.Background(), bundle, key)
	if err != nil || !replay {
		t.Fatalf("重启后幂等重放失败: replay=%v err=%v", replay, err)
	}
	if secondReplay.Permit.SpaceID != durableReplay.Permit.SpaceID || secondReplay.Permit.Workers[0].Name != durableReplay.Permit.Workers[0].Name {
		t.Fatalf("幂等重放缓存被调用方修改污染: memory=(%q,%q) durable=(%q,%q)", secondReplay.Permit.SpaceID, secondReplay.Permit.Workers[0].Name, durableReplay.Permit.SpaceID, durableReplay.Permit.Workers[0].Name)
	}
}
