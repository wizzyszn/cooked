package errors

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Code       string      `json:"code"`
	Message    string      `json:"message"`
	HTTPStatus int         `json:"-"`
	Details    interface{} `json:"details;omitempty"`
}

func New(code, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Wrap(err error, code string, httpStatus int) *AppError {

	return &AppError{
		Code:       code,
		Message:    err.Error(),
		HTTPStatus: httpStatus,
	}
}

func WithMessage(baseError *AppError, message string) *AppError {
	return &AppError{
		Code:       baseError.Code,
		Message:    fmt.Sprintf("%s: %s", baseError.Message, message),
		HTTPStatus: baseError.HTTPStatus,
	}
}

func WithDetails(baseError *AppError, details interface{}) *AppError {
	return &AppError{
		Code:       baseError.Code,
		Message:    baseError.Message,
		HTTPStatus: baseError.HTTPStatus,
		Details:    details,
	}
}

var (
	ErrNotFound            = New("NOT_FOUND", "Resource not found", http.StatusNotFound)
	ErrValidation          = New("VALIDATION_ERROR", "Invalid input", http.StatusBadRequest)
	ErrTooManyRequests     = New("TOO_MANY_REQUESTS", "Too many requests, please try again later", http.StatusTooManyRequests)
	ErrUnAuthorized        = New("UNAUTHORIZED", "Authentication required", http.StatusUnauthorized)
	ErrForbidden           = New("FORBIDDEN", "Insufficient permissions", http.StatusForbidden)
	ErrTokenExpired        = New("TOKEN_EXPIRED", "Authentication token has expired", http.StatusUnauthorized)
	ErrInvalidToken        = New("INVALID_TOKEN", "Authentication token is invalid", http.StatusUnauthorized)
	ErrBadRequest          = New("BAD_REQUEST", "Bad request", http.StatusBadRequest)
	ErrServiceUnavailable  = New("SERVICE_UNAVAILABLE", "Service is unavailable at the moment", http.StatusServiceUnavailable)
	ErrInternalServerError = New("INTERNAL_SERVER_ERROR", "An internal server error occured", http.StatusInternalServerError)
)
