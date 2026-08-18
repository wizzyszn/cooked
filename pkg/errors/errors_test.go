package errors

import (
	"fmt"
	"strings"
	"testing"
)

func TestWrapKeepsPublicMessage(t *testing.T) {
	cause := fmt.Errorf("pq: relation users does not exist")
	got := ErrInternalServerError.Wrap(cause, ErrInternalServerError.Code, ErrInternalServerError.HTTPStatus)
	if strings.Contains(got.Message, "pq:") || strings.Contains(got.Message, "does not exist") {
		t.Fatalf("public message leaked cause: %q", got.Message)
	}
	if got.Message != ErrInternalServerError.Message {
		t.Fatalf("message = %q, want %q", got.Message, ErrInternalServerError.Message)
	}
	if got.Unwrap() != cause {
		t.Fatalf("unwrap = %v, want %v", got.Unwrap(), cause)
	}
}
