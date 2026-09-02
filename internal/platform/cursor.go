package platform

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Cursor struct {
	Timestamp time.Time `json:"timestamp"`
	ID        uuid.UUID `json:"id"`
}

func EncodeCursor(cursor Cursor) (string, error) {
	if cursor.Timestamp.IsZero() || cursor.ID == uuid.Nil {
		return "", fmt.Errorf("cursor timestamp and id are required")
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecodeCursor(raw string) (Cursor, error) {
	var cursor Cursor
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursor, fmt.Errorf("decode cursor: %w", err)
	}
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return cursor, fmt.Errorf("parse cursor: %w", err)
	}
	if cursor.Timestamp.IsZero() || cursor.ID == uuid.Nil {
		return Cursor{}, fmt.Errorf("cursor timestamp and id are required")
	}
	return cursor, nil
}
