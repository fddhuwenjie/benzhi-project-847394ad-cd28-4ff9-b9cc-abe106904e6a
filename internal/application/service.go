package application

import (
	"context"
	"time"
)

type Service struct {
	repo    Repository
	clock   Clock
	cursors *cursorRegistry
}

func New(repo Repository) *Service {
	return &Service{repo: repo, clock: realClock{}, cursors: newCursorRegistry()}
}

func NewWithClock(repo Repository, clock Clock) *Service {
	return &Service{repo: repo, clock: clock, cursors: newCursorRegistry()}
}

func (s *Service) Flush(ctx context.Context) error { return s.repo.Flush(ctx) }

func (s *Service) now() time.Time { return s.clock.Now().UTC() }
