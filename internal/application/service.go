package application

import (
	"context"
	"sync"
	"time"

	"confinedpermit/internal/domain"
)

type Service struct {
	repo            Repository
	clock           Clock
	collectionMu    sync.RWMutex
	collection      []*domain.PermitBundle
	collectionReady bool
}

func New(repo Repository) *Service { return &Service{repo: repo, clock: realClock{}} }

func NewWithClock(repo Repository, clock Clock) *Service { return &Service{repo: repo, clock: clock} }

func (s *Service) Flush(ctx context.Context) error { return s.repo.Flush(ctx) }

func (s *Service) now() time.Time { return s.clock.Now().UTC() }

// create persists a new permit bundle and invalidates the cached
// collection snapshot so subsequent queue reads observe the new state.
// Idempotent replays do not mutate storage and therefore skip invalidation.
func (s *Service) create(ctx context.Context, b *domain.PermitBundle, key IdempotencyKey) (*domain.PermitBundle, bool, error) {
	saved, replay, err := s.repo.Create(ctx, b, key)
	if err == nil && !replay {
		s.invalidateCollection()
	}
	return saved, replay, err
}

// mutate applies a mutation to an existing permit bundle and invalidates
// the cached collection snapshot so subsequent queue reads observe the
// updated state. Idempotent replays do not mutate storage and therefore
// skip invalidation.
func (s *Service) mutate(ctx context.Context, id string, expected int64, key IdempotencyKey, fn func(*domain.PermitBundle) error) (*domain.PermitBundle, bool, error) {
	saved, replay, err := s.repo.Mutate(ctx, id, expected, key, fn)
	if err == nil && !replay {
		s.invalidateCollection()
	}
	return saved, replay, err
}

// invalidateCollection drops the cached permit collection so that the next
// queue query reloads the current persisted state. It is called after any
// successful write that changes storage.
func (s *Service) invalidateCollection() {
	s.collectionMu.Lock()
	defer s.collectionMu.Unlock()
	s.collection = nil
	s.collectionReady = false
}

func (s *Service) collectionSnapshot(ctx context.Context, repo CollectionRepository) ([]*domain.PermitBundle, error) {
	s.collectionMu.RLock()
	if s.collectionReady {
		bundles := s.collection
		s.collectionMu.RUnlock()
		return bundles, nil
	}
	s.collectionMu.RUnlock()

	bundles, err := repo.List(ctx)
	if err != nil {
		return nil, err
	}
	s.collectionMu.Lock()
	if !s.collectionReady {
		s.collection = bundles
		s.collectionReady = true
	} else {
		bundles = s.collection
	}
	s.collectionMu.Unlock()
	return bundles, nil
}
