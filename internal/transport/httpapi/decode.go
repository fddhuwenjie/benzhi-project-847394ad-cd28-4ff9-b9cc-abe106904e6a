package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxRequestBody = 1 << 20

type protocolError struct {
	Status  int
	Code    string
	Message string
}

func (e *protocolError) Error() string { return e.Message }

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	mediaType := strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])
	if mediaType != "application/json" {
		return &protocolError{Status: http.StatusUnsupportedMediaType, Code: "CONTENT_TYPE_REQUIRED", Message: "Content-Type 必须为 application/json"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return &protocolError{Status: http.StatusRequestEntityTooLarge, Code: "BODY_TOO_LARGE", Message: "请求体不能超过 1 MiB"}
		}
		return &protocolError{Status: http.StatusBadRequest, Code: "INVALID_JSON", Message: friendlyJSONError(err)}
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return &protocolError{Status: http.StatusBadRequest, Code: "INVALID_JSON", Message: "请求体只能包含一个 JSON 对象"}
	}
	return nil
}

func friendlyJSONError(err error) string {
	var syntax *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return fmt.Sprintf("JSON 语法错误，位置 %d", syntax.Offset)
	case errors.As(err, &typeErr):
		return "字段 " + typeErr.Field + " 的类型不正确"
	case errors.Is(err, io.EOF):
		return "请求体不能为空"
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		return "请求包含未知字段 " + strings.TrimPrefix(err.Error(), "json: unknown field ")
	default:
		return "无法解析 JSON 请求体"
	}
}

func permitID(r *http.Request) (string, error) {
	id := strings.TrimSpace(r.PathValue("permit_id"))
	if id == "" {
		return "", &protocolError{Status: http.StatusBadRequest, Code: "PERMIT_ID_REQUIRED", Message: "许可标识不能为空"}
	}
	if len(id) > 128 || strings.ContainsAny(id, "/\\\x00") {
		return "", &protocolError{Status: http.StatusBadRequest, Code: "PERMIT_ID_INVALID", Message: "许可标识格式无效"}
	}
	return id, nil
}
