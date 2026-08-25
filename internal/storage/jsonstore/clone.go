package jsonstore

import (
	"encoding/json"
	"fmt"

	"confinedpermit/internal/domain"
)

func cloneBundle(in *domain.PermitBundle) (*domain.PermitBundle, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("复制许可数据: %w", err)
	}
	var out domain.PermitBundle
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("复制许可数据: %w", err)
	}
	return &out, nil
}

func cloneDocumentWithBundle(d *document, id string, bundle *domain.PermitBundle, idemKey string, idem idempotencyRecord) *document {
	next := &document{Version: d.Version, Permits: d.Permits, Idempotency: d.Idempotency}
	next.Permits[id] = bundle
	next.Idempotency[idemKey] = idem
	return next
}
