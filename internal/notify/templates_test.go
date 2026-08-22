package notify

import (
	"strings"
	"testing"
)

func TestRenderVerifyEmail(t *testing.T) {
	got := Render(TemplateVerifyEmail, map[string]any{
		"name":       "Ada",
		"verify_url": "https://cooked.example/api/v1/auth/verify-email?token=abc",
	})
	if got == nil {
		t.Fatal("expected rendered verify_email template")
	}
	if got.Title == "" || got.Body == "" || got.HTML == "" {
		t.Fatalf("incomplete render: %+v", got)
	}
	want := "https://cooked.example/api/v1/auth/verify-email?token=abc"
	if !strings.Contains(got.HTML, want) || !strings.Contains(got.Body, want) {
		t.Fatalf("missing verify url in render: body=%q html=%q", got.Body, got.HTML)
	}
	if !strings.Contains(got.Body, "Ada") {
		t.Fatalf("missing name in body: %q", got.Body)
	}
}

func TestRenderForgotPassOtp(t *testing.T) {
	got := Render(TemplateForgotPassOtp, map[string]any{
		"name": "Ada",
		"otp":  "123456",
	})
	if got == nil {
		t.Fatal("expected rendered forgot_otp template")
	}
	if got.Title == "" || got.Body == "" || got.HTML == "" {
		t.Fatalf("incomplete render: %+v", got)
	}
	if !strings.Contains(got.Body, "123456") || !strings.Contains(got.HTML, "123456") {
		t.Fatalf("missing otp in render: body=%q html=%q", got.Body, got.HTML)
	}
	if !strings.Contains(got.Body, "Ada") {
		t.Fatalf("missing name in body: %q", got.Body)
	}
}

func TestRenderUnknown(t *testing.T) {
	if got := Render("nope", nil); got != nil {
		t.Fatalf("expected nil for unknown template, got %+v", got)
	}
}
