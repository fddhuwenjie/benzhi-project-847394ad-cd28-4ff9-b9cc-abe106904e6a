package domain

import "fmt"

type ErrorKind string

const (
	KindValidation ErrorKind = "validation"
	KindConflict   ErrorKind = "conflict"
	KindNotFound   ErrorKind = "not_found"
	KindInternal   ErrorKind = "internal"
)

type BusinessError struct {
	Kind    ErrorKind `json:"kind"`
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Issues  []Issue   `json:"issues,omitempty"`
}

func (e *BusinessError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func NewValidation(code, message string, issues []Issue) error {
	return &BusinessError{Kind: KindValidation, Code: code, Message: message, Issues: issues}
}

func NewConflict(code, message string) error {
	return &BusinessError{Kind: KindConflict, Code: code, Message: message}
}

func NewNotFound(resource string) error {
	return &BusinessError{Kind: KindNotFound, Code: "NOT_FOUND", Message: resource + "不存在"}
}

func NewNotFoundCode(code, message string) error {
	return &BusinessError{Kind: KindNotFound, Code: code, Message: message}
}

func NewInternal(message string) error {
	return &BusinessError{Kind: KindInternal, Code: "INTERNAL_ERROR", Message: message}
}

func AsBusiness(err error) (*BusinessError, bool) {
	be, ok := err.(*BusinessError)
	return be, ok
}
