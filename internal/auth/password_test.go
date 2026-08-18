package auth

import "testing"

func TestPasswordMatches(t *testing.T) {
	hash, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !passwordMatches(hash, "correct-horse") {
		t.Fatal("expected match")
	}
	if passwordMatches(hash, "wrong-password") {
		t.Fatal("expected mismatch")
	}
	if passwordMatches("", "correct-horse") {
		t.Fatal("empty hash must not match")
	}
	if passwordMatches("", "timing-dummy") {
		t.Fatal("empty hash must not match the dummy password")
	}
	if passwordMatches("not-a-hash", "correct-horse") {
		t.Fatal("unparseable hash must not match")
	}
}

func TestHashRefreshToken(t *testing.T) {
	raw := "refresh.jwt.token"
	got := HashRefreshToken(raw)
	if got == raw {
		t.Fatal("hash must not equal the raw token")
	}
	if len(got) != 64 {
		t.Fatalf("expected sha256 hex, got %q", got)
	}
	if HashRefreshToken(raw) != got {
		t.Fatal("hash must be deterministic")
	}
}
