package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"

	"confinedpermit/internal/domain"
)

var technicalIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@-]{0,127}$`)

func makeKey(scope, action string, meta RequestMeta, command any) (IdempotencyKey, error) {
	if err := validateIdentifier("request_id", meta.RequestID); err != nil {
		return IdempotencyKey{}, err
	}
	if err := validateIdentifier("actor_id", meta.ActorID); err != nil {
		return IdempotencyKey{}, err
	}
	b, err := json.Marshal(command)
	if err != nil {
		return IdempotencyKey{}, domain.NewInternal("无法计算请求摘要")
	}
	h := sha256.Sum256(b)
	return IdempotencyKey{Scope: scope, Action: action, RequestID: meta.RequestID, Digest: hex.EncodeToString(h[:])}, nil
}

func validateIdentifier(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return domain.NewValidation(strings.ToUpper(field)+"_REQUIRED", field+" 不能为空", nil)
	}
	if !technicalIdentifier.MatchString(value) {
		return domain.NewValidation(strings.ToUpper(field)+"_INVALID", field+" 必须为 1 至 128 位技术标识，只能包含字母、数字、点、下划线、冒号、@ 或连字符", nil)
	}
	return nil
}

func requireRevision(meta RequestMeta) error {
	if meta.ExpectedRevision < 1 {
		return domain.NewValidation("REVISION_REQUIRED", "expected_revision 必须大于零", nil)
	}
	return nil
}
