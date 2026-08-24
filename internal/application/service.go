package application

import (
	"context"
	"time"
)

type Service struct {
	repo  Repository
	clock Clock
}

func New(repo Repository) *Service { return &Service{repo: repo, clock: realClock{}} }

func NewWithClock(repo Repository, clock Clock) *Service { return &Service{repo: repo, clock: clock} }

func (s *Service) Flush(ctx context.Context) error { return s.repo.Flush(ctx) }

func (s *Service) now() time.Time { return s.clock.Now().UTC() }
