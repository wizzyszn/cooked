package cook

import (
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/platform"
	"testing"
)

func TestClientAnalyticsAllowlist(t *testing.T) {
	svc := &Service{clock: platform.RealClock{}}
	user := uuid.New()
	for _, req := range []EventRequest{{EventName: "unknown"}, {EventName: "recipe_viewed", Properties: map[string]any{"email": "private@example.com"}}, {EventName: "recipe_viewed", Properties: map[string]any{"source": map[string]any{"nested": "value"}}}} {
		if e := svc.Ingest(t.Context(), &user, req); e == nil {
			t.Fatalf("accepted %#v", req)
		}
	}
}
