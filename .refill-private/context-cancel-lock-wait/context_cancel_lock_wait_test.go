package context_cancel_lock_wait_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
	"confinedpermit/internal/storage/jsonstore"
)

type observedContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *observedContext) Err() error {
	err := c.Context.Err()
	c.once.Do(func() { close(c.checked) })
	return err
}

type mutateResult struct {
	err error
}

func TestCanceledMutationStopsWhilePermitLockHeld(t *testing.T) {
	store, err := jsonstore.Open(filepath.Join(t.TempDir(), "permits.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	permit := domain.NewPermit("permit-context-lock", domain.DraftData{
		SpaceID:      "SPACE-CONTEXT",
		PlannedStart: now.Add(time.Hour),
		PlannedEnd:   now.Add(2 * time.Hour),
	}, now)
	_, _, err = store.Create(context.Background(), &domain.PermitBundle{Permit: permit}, application.IdempotencyKey{
		Scope: "permits", Action: "create", RequestID: "context-create", Digest: "context-create-digest",
	})
	if err != nil {
		t.Fatal(err)
	}

	holderEntered := make(chan struct{})
	releaseHolder := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		_, _, mutateErr := store.Mutate(context.Background(), permit.ID, 1, application.IdempotencyKey{
			Scope: permit.ID, Action: "holder", RequestID: "context-holder", Digest: "context-holder-digest",
		}, func(bundle *domain.PermitBundle) error {
			close(holderEntered)
			<-releaseHolder
			bundle.Permit.Status = domain.StatusPendingReview
			return nil
		})
		holderDone <- mutateErr
	}()
	<-holderEntered

	parent, cancel := context.WithCancel(context.Background())
	ctx := &observedContext{Context: parent, checked: make(chan struct{})}
	waiterDone := make(chan mutateResult, 1)
	go func() {
		_, _, mutateErr := store.Mutate(ctx, permit.ID, 1, application.IdempotencyKey{
			Scope: permit.ID, Action: "waiter", RequestID: "context-waiter", Digest: "context-waiter-digest",
		}, func(*domain.PermitBundle) error {
			return errors.New("已取消的变更不应进入领域回调")
		})
		waiterDone <- mutateResult{err: mutateErr}
	}()
	<-ctx.checked
	cancel()

	result := <-waiterDone
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("等待许可锁期间取消后应返回 context.Canceled，实际为 %v", result.err)
	}
	close(releaseHolder)
	if err := <-holderDone; err != nil {
		t.Fatalf("持锁变更失败: %v", err)
	}
}
