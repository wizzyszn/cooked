package auth

import "testing"

func TestAllowedReturnURLRequiresExactConfiguredURL(t *testing.T) {
	allowed := []string{"https://app.example.com/auth/callback"}
	for _, tt := range []struct {
		raw  string
		want bool
	}{
		{"https://app.example.com/auth/callback", true},
		{"https://app.example.com/auth/callback?next=/admin", false},
		{"https://evil.example/auth/callback", false},
		{"javascript:alert(1)", false},
	} {
		if got := allowedReturnURL(tt.raw, allowed); got != tt.want {
			t.Errorf("allowedReturnURL(%q)=%v want %v", tt.raw, got, tt.want)
		}
	}
}

func TestAppendQueryPreservesExistingParameters(t *testing.T) {
	got := appendQuery("https://app.example/callback?source=google", "code", "secret")
	if got != "https://app.example/callback?code=secret&source=google" {
		t.Fatalf("result = %q", got)
	}
}
