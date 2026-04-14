package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

type AppError struct {
	Code   int               `json:"-"`
	Msg    string            `json:"error"`
	Fields map[string]string `json:"fields,omitempty"`
	Inner  error             `json:"-"`
}

func (e *AppError) Error() string {
	if e.Inner != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Inner)
	}
	return e.Msg
}

func (e *AppError) Unwrap() error {
	return e.Inner
}

func NewValidation(fields map[string]string) *AppError {
	return &AppError{
		Code:   http.StatusBadRequest,
		Msg:    "validation failed",
		Fields: fields,
	}
}

func NewBadRequest(msg string) *AppError {
	return &AppError{Code: http.StatusBadRequest, Msg: msg}
}

func NewUnauthorized(msg string) *AppError {
	return &AppError{Code: http.StatusUnauthorized, Msg: msg}
}

func NewForbidden(msg string) *AppError {
	return &AppError{Code: http.StatusForbidden, Msg: msg}
}

func NewNotFound(msg string) *AppError {
	return &AppError{Code: http.StatusNotFound, Msg: msg}
}

func NewConflict(msg string) *AppError {
	return &AppError{Code: http.StatusConflict, Msg: msg}
}

func NewTooManyRequests(msg string) *AppError {
	return &AppError{Code: http.StatusTooManyRequests, Msg: msg}
}

func NewInternal(inner error) *AppError {
	return &AppError{
		Code:  http.StatusInternalServerError,
		Msg:   "internal server error",
		Inner: inner,
	}
}

func AsAppError(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}
