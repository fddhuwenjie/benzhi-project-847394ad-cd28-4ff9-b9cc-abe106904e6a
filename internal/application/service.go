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
