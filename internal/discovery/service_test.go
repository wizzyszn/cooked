package discovery

import "testing"

func TestNormalizeFilters(t *testing.T) {
	f, err := normalizeFilters(Filters{})
	if err != nil || f.Limit != 20 {
		t.Fatalf("default=%#v err=%v", f, err)
	}
	for _, bad := range []Filters{{Limit: 51}, {Difficulty: "expert"}, {MaxSeconds: intPtr(-1)}, {Cursor: "broken"}} {
		if _, err = normalizeFilters(bad); err == nil {
			t.Fatalf("accepted invalid filters %#v", bad)
		}
	}
}

func intPtr(v int) *int { return &v }
