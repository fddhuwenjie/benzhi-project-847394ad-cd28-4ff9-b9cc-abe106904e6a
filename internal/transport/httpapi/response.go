package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"confinedpermit/internal/domain"
)

type responseEnvelope struct {
	Data any `json:"data"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id,omitempty"`
	Issues    []domain.Issue `json:"issues,omitempty"`
}

func writeData(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(responseEnvelope{Data: data})
}

func writeError(w http.ResponseWriter, err error, requestID string) {
	status := http.StatusInternalServerError
	body := errorBody{Code: "INTERNAL_ERROR", Message: "服务处理请求时发生错误", RequestID: requestID}
	var pe *protocolError
	if errors.As(err, &pe) {
		status = pe.Status
		body.Code = pe.Code
		body.Message = pe.Message
	} else if be, ok := domain.AsBusiness(err); ok {
		body.Code = be.Code
		body.Message = be.Message
		body.Issues = be.Issues
		switch be.Kind {
		case domain.KindValidation:
			status = http.StatusUnprocessableEntity
		case domain.KindConflict:
			status = http.StatusConflict
		case domain.KindNotFound:
			status = http.StatusNotFound
		default:
			status = http.StatusInternalServerError
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{Error: body})
}
