package platform

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	want := Cursor{Timestamp: time.Date(2026, 9, 2, 12, 30, 0, 0, time.UTC), ID: uuid.New()}
	raw, err := EncodeCursor(want)
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}
	got, err := DecodeCursor(raw)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if !got.Timestamp.Equal(want.Timestamp) || got.ID != want.ID {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestCursorRejectsMalformedValue(t *testing.T) {
	if _, err := DecodeCursor("not-a-cursor"); err == nil {
		t.Fatal("expected malformed cursor to fail")
	}
}

func TestParseIdempotencyKey(t *testing.T) {
	if got, err := ParseIdempotencyKey(" cook-session:123 "); err != nil || got != "cook-session:123" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, invalid := range []string{"short", "spaces are rejected", stringsOfLength(MaxIdempotencyKeyLength + 1)} {
		if _, err := ParseIdempotencyKey(invalid); err == nil {
			t.Fatalf("expected %q to fail", invalid)
		}
	}
}

func TestClockAndIdentifierImplementations(t *testing.T) {
	want := time.Date(2026, 9, 2, 18, 0, 0, 0, time.UTC)
	if got := (FixedClock{Time: want}).Now(); !got.Equal(want) {
		t.Fatalf("fixed clock = %v, want %v", got, want)
	}
	if got := (RealClock{}).Now(); got.Location() != time.UTC {
		t.Fatalf("real clock location = %v, want UTC", got.Location())
	}
	if got := (UUIDGenerator{}).New(); got == uuid.Nil {
		t.Fatal("UUID generator returned nil UUID")
	}
}

func stringsOfLength(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
