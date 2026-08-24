package application

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	"confinedpermit/internal/domain"
)

type cursorPayload struct {
	Scope  string `json:"scope"`
	Filter string `json:"filter"`
	Offset int    `json:"offset"`
}

func filterDigest(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func encodeCursor(scope, filter string, offset int) string {
	b, _ := json.Marshal(cursorPayload{Scope: scope, Filter: filter, Offset: offset})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(raw, scope, filter string) (int, error) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, domain.NewValidation("CURSOR_INVALID", "cursor 已损坏或不属于当前查询", nil)
	}
	var payload cursorPayload
	if json.Unmarshal(b, &payload) != nil || payload.Scope != scope || payload.Filter != filter || payload.Offset < 0 {
		return 0, domain.NewValidation("CURSOR_INVALID", "cursor 已损坏或不属于当前查询", nil)
	}
	return payload.Offset, nil
}
