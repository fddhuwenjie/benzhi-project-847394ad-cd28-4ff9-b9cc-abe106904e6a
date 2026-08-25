package application

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"sync"

	"confinedpermit/internal/domain"
)

type cursorPayload struct {
	Scope  string `json:"scope"`
	Filter string `json:"filter"`
	Offset int    `json:"offset"`
}

type cursorRegistry struct {
	mu      sync.RWMutex
	entries map[string]cursorPayload
}

func newCursorRegistry() *cursorRegistry {
	return &cursorRegistry{entries: make(map[string]cursorPayload)}
}

func filterDigest(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (r *cursorRegistry) encode(scope, filter string, offset int) string {
	token := newID("page")
	r.mu.Lock()
	r.entries[token] = cursorPayload{Scope: scope, Filter: filter, Offset: offset}
	r.mu.Unlock()
	return base64.RawURLEncoding.EncodeToString([]byte(token))
}

func (r *cursorRegistry) decode(raw, scope, filter string) (int, error) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, domain.NewValidation("CURSOR_INVALID", "cursor 已损坏或不属于当前查询", nil)
	}
	r.mu.RLock()
	payload, ok := r.entries[string(b)]
	r.mu.RUnlock()
	if !ok || payload.Scope != scope || payload.Filter != filter || payload.Offset < 0 {
		return 0, domain.NewValidation("CURSOR_INVALID", "cursor 已损坏或不属于当前查询", nil)
	}
	return payload.Offset, nil
}
