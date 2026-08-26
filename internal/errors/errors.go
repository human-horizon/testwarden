// Package errors provides structured error types with codes for testwarden.
package errors

import (
	"errors"
	"fmt"
)

// Code categorises errors for programmatic handling.
type Code string

const (
	CodeConfig     Code = "CONFIG"
	CodeCoverage   Code = "COVERAGE"
	CodeAI         Code = "AI"
	CodeFilesystem Code = "FILESYSTEM"
	CodeNetwork    Code = "NETWORK"
	CodeValidation Code = "VALIDATION"
	CodeGit        Code = "GIT"
)

// Error is a structured error with code and cause.
type Error struct {
	Code    Code
	Message string
	Cause   error
	Context map[string]any
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *Error) Unwrap() error { return e.Cause }

// WithContext attaches key/value metadata to the error.
func (e *Error) WithContext(key string, value any) *Error {
	if e.Context == nil {
		e.Context = make(map[string]any)
	}
	e.Context[key] = value
	return e
}

// New creates a new Error with the given code and message.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap creates a new Error wrapping an underlying cause.
func Wrap(code Code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

// CodeOf returns the Code of err if it is (or wraps) a *Error.
func CodeOf(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// IsCode reports whether err is (or wraps) an Error with the given code.
func IsCode(err error, code Code) bool {
	return CodeOf(err) == code
}
