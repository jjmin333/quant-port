package toss

import (
	"errors"
	"fmt"
	"net/http"
)

const ErrCodeInvalidToken = "invalid-token"

type Error struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
}

func (e *Error) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("toss api %d %s: %s (requestId=%s)", e.StatusCode, e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("toss api %d %s: %s", e.StatusCode, e.Code, e.Message)
}

func newErrorFromAPI(status int, body ErrorBody) *Error {
	return &Error{
		StatusCode: status,
		Code:       body.Error.Code,
		Message:    body.Error.Message,
		RequestID:  body.Error.RequestID,
	}
}

func IsInvalidTokenError(err error) bool {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusUnauthorized || apiErr.Code == ErrCodeInvalidToken
	}
	return false
}
