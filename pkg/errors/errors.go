package errors

import (
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

type AppError struct {
	Code       string      `json:"code"`
	Message    string      `json:"message"`
	HTTPStatus int         `json:"-"`
	Details    interface{} `json:"details,omitempty"`
	cause      error       `json:"-"`
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

func (e *AppError) Unwrap() error {
	return e.cause
}

func (e *AppError) Wrap(err error, code string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    e.Message,
		HTTPStatus: httpStatus,
		cause:      err,
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
func Internal(log *zap.SugaredLogger, msg string, err error, keysAndValues ...interface{}) error {
	if log != nil {
		args := append([]interface{}{"error", err}, keysAndValues...)
		log.Errorw(msg, args...)
	}
	return ErrInternalServerError
}

var (
	ErrNotFound               = New("NOT_FOUND", "Resource not found", http.StatusNotFound)
	ErrValidation             = New("VALIDATION_ERROR", "Invalid input", http.StatusBadRequest)
	ErrTooManyRequests        = New("TOO_MANY_REQUESTS", "Too many requests, please try again later", http.StatusTooManyRequests)
	ErrUnauthorized           = New("UNAUTHORIZED", "Authentication required", http.StatusUnauthorized)
	ErrUnAuthorized           = ErrUnauthorized // Deprecated: use ErrUnauthorized.
	ErrForbidden              = New("FORBIDDEN", "Insufficient permissions", http.StatusForbidden)
	ErrTokenExpired           = New("TOKEN_EXPIRED", "Authentication token has expired", http.StatusUnauthorized)
	ErrInvalidToken           = New("INVALID_TOKEN", "Authentication token is invalid", http.StatusUnauthorized)
	ErrBadRequest             = New("BAD_REQUEST", "Bad request", http.StatusBadRequest)
	ErrServiceUnavailable     = New("SERVICE_UNAVAILABLE", "Service is unavailable at the moment", http.StatusServiceUnavailable)
	ErrInternalServerError    = New("INTERNAL_SERVER_ERROR", "An internal server error occurred", http.StatusInternalServerError)
	ErrConflict               = New("CONFLICT_ERROR", "Conflict error", http.StatusConflict)
	ErrEmailTaken             = New("EMAIL_TAKEN", "an account with this email already exists", http.StatusConflict)
	ErrUsernameTaken          = New("USERNAME_TAKEN", "this username is already taken", http.StatusConflict)
	ErrInvalidEmailOrPassword = New("INVALID_EMAIL_OR_PASSWORD", "Invalid email or password", http.StatusUnauthorized)
	ErrEmailNotVerified       = New("EMAIL_NOT_VERIFIED", "Please confirm your email before signing in", http.StatusForbidden)
)
