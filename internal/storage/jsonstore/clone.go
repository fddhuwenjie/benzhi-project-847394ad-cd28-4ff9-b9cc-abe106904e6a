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
	permits := make(map[string]*domain.PermitBundle, len(d.Permits)+1)
	for k, v := range d.Permits {
		permits[k] = v
	}
	idempotency := make(map[string]idempotencyRecord, len(d.Idempotency)+1)
	for k, v := range d.Idempotency {
		idempotency[k] = v
	}
	permits[id] = bundle
	idempotency[idemKey] = idem
	return &document{Version: d.Version, Permits: permits, Idempotency: idempotency}
}
