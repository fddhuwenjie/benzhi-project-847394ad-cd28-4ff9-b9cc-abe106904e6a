package jsonstore

import (
	"encoding/json"
	"time"

	"confinedpermit/internal/domain"
)

const documentVersion = 1

type document struct {
	Version     int                             `json:"version"`
	Permits     map[string]*domain.PermitBundle `json:"permits"`
	Idempotency map[string]idempotencyRecord    `json:"idempotency"`
}

type idempotencyRecord struct {
	Digest    string          `json:"digest"`
	Result    json.RawMessage `json:"result"`
	CreatedAt time.Time       `json:"created_at"`
}

func emptyDocument() *document {
	return &document{Version: documentVersion, Permits: map[string]*domain.PermitBundle{}, Idempotency: map[string]idempotencyRecord{}}
}
