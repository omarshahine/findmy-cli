package ledger

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndQueryAllKinds(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)

	obs := []Observation{
		testObservation(base, "people", "Sadie Van Horn"),
		testObservation(base.Add(time.Minute), "devices", "Omar's iPhone"),
		testObservation(base.Add(2*time.Minute), "items", "Backpack"),
	}
	if err := l.Append(ctx, obs); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	for _, kind := range []string{"people", "devices", "items"} {
		got, err := l.Query(ctx, QueryOptions{Kind: kind})
		if err != nil {
			t.Fatalf("Query(%q) error = %v", kind, err)
		}
		if len(got) != 1 {
			t.Fatalf("Query(%q) returned %d rows, want 1", kind, len(got))
		}
		if got[0].Kind != kind {
			t.Fatalf("Query(%q) kind = %q", kind, got[0].Kind)
		}
		if !json.Valid(got[0].Raw) {
			t.Fatalf("Query(%q) raw JSON is invalid: %s", kind, got[0].Raw)
		}
	}
}

func TestQueryNameExactThenSubstringFallback(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)

	if err := l.Append(ctx, []Observation{
		testObservation(base, "people", "Sadie"),
		testObservation(base.Add(time.Minute), "people", "Sadie Van Horn"),
		testObservation(base.Add(2*time.Minute), "people", "Ava Van Horn"),
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	exact, err := l.Query(ctx, QueryOptions{Name: "Sadie"})
	if err != nil {
		t.Fatalf("Query exact error = %v", err)
	}
	if len(exact) != 1 || exact[0].Name != "Sadie" {
		t.Fatalf("Query exact returned %#v, want only Sadie", exact)
	}

	substring, err := l.Query(ctx, QueryOptions{Name: "Van Horn"})
	if err != nil {
		t.Fatalf("Query substring error = %v", err)
	}
	if len(substring) != 2 {
		t.Fatalf("Query substring returned %d rows, want 2", len(substring))
	}
	if substring[0].Name != "Sadie Van Horn" || substring[1].Name != "Ava Van Horn" {
		t.Fatalf("Query substring order/names = %#v", substring)
	}
}

func TestQueryTimeRangeAndLimit(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)

	if err := l.Append(ctx, []Observation{
		testObservation(base, "people", "Sadie"),
		testObservation(base.Add(time.Hour), "people", "Sadie"),
		testObservation(base.Add(2*time.Hour), "people", "Sadie"),
		testObservation(base.Add(3*time.Hour), "people", "Sadie"),
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	ranged, err := l.Query(ctx, QueryOptions{
		Name:  "Sadie",
		Since: base.Add(time.Hour),
		Until: base.Add(3 * time.Hour),
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("Query range error = %v", err)
	}
	if len(ranged) != 2 {
		t.Fatalf("Query range returned %d rows, want 2", len(ranged))
	}
	if !ranged[0].Ts.Equal(base.Add(2*time.Hour)) || !ranged[1].Ts.Equal(base.Add(3*time.Hour)) {
		t.Fatalf("Query limit should keep most recent rows in ascending order, got %#v", ranged)
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("FINDMY_HISTORY_DB", "/tmp/findmy-test.sqlite")
	if got := DefaultPath(); got != "/tmp/findmy-test.sqlite" {
		t.Fatalf("DefaultPath with override = %q", got)
	}

	t.Setenv("FINDMY_HISTORY_DB", "")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	want := filepath.Join("/tmp/xdg", "findmy-cli", "history.sqlite")
	if got := DefaultPath(); got != want {
		t.Fatalf("DefaultPath with XDG_DATA_HOME = %q, want %q", got, want)
	}
}

func openTestLedger(t *testing.T) *Ledger {
	t.Helper()
	l, err := Open(filepath.Join(t.TempDir(), "history.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := l.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return l
}

func testObservation(ts time.Time, kind, name string) Observation {
	raw, _ := json.Marshal(map[string]string{
		"name":     name,
		"location": "Bellevue, WA",
	})
	return Observation{
		Ts:        ts,
		Kind:      kind,
		Name:      name,
		Location:  "Bellevue, WA",
		Staleness: "2 min. ago",
		Distance:  "1.5 mi",
		Battery:   "82%",
		Raw:       raw,
	}
}
