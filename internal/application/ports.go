package application

import (
	"context"
	"time"

	"confinedpermit/internal/domain"
)

type IdempotencyKey struct {
	Scope     string `json:"scope"`
	Action    string `json:"action"`
	RequestID string `json:"request_id"`
	Digest    string `json:"digest"`
}

type Repository interface {
	Create(context.Context, *domain.PermitBundle, IdempotencyKey) (*domain.PermitBundle, bool, error)
	Mutate(context.Context, string, int64, IdempotencyKey, func(*domain.PermitBundle) error) (*domain.PermitBundle, bool, error)
	Get(context.Context, string) (*domain.PermitBundle, error)
	Flush(context.Context) error
}

type CollectionRepository interface {
	List(context.Context) ([]*domain.PermitBundle, error)
}

type Clock interface{ Now() time.Time }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }
