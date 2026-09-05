// Package apperr defines errors that retain a stable machine-readable code.
package apperr

import "fmt"

type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

func Wrap(code, message string, cause error) error {
	return &Error{Code: code, Message: message, Cause: cause}
}
