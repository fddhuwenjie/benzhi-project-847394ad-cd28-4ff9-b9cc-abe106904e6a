package contextcancelaftercallback

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
	"confinedpermit/internal/storage/jsonstore"
)

func TestCanceledMutationDoesNotCommitAfterContextCancel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permits.json")
	store, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	bundle := &domain.PermitBundle{Permit: domain.NewPermit("cancel-after-callback", domain.DraftData{
		SpaceID:      "space-1",
		PlannedStart: now,
		PlannedEnd:   now.Add(time.Hour),
	}, now)}
	createKey := application.IdempotencyKey{Scope: "permits", Action: "create", RequestID: "create-cancel-after-callback", Digest: "digest-create"}
	if _, _, err := store.Create(context.Background(), bundle, createKey); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	release := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, _, err := store.Mutate(ctx, bundle.Permit.ID, 1, application.IdempotencyKey{
			Scope: bundle.Permit.ID, Action: "cancel-after-callback", RequestID: "mutate-cancel-after-callback", Digest: "digest-mutate",
		}, func(work *domain.PermitBundle) error {
			close(entered)
			<-release
			work.Permit.Status = domain.StatusPendingReview
			return nil
		})
		result <- err
	}()

	<-entered
	cancel()
	close(release)
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("取消后的 Mutate 应返回 context.Canceled，实际为 %v", err)
	}

	reopened, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get(context.Background(), bundle.Permit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Permit.Revision != 1 || got.Permit.Status != domain.StatusDraft {
		t.Fatalf("取消后的变更不应持久化，实际 revision=%d status=%s", got.Permit.Revision, got.Permit.Status)
	}
}
