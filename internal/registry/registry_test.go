package registry

import (
	"io"
	"log/slog"
	"testing"

	"github.com/liskl/coolledux-controller/internal/config"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testCfg() *config.Config {
	return &config.Config{
		Display: config.DisplayConfig{Columns: 96, Rows: 16},
	}
}

func TestAdd_Get_Primary(t *testing.T) {
	r := New()
	a := BuildEntry(config.DeviceConfig{Name: "a", MAC: "01:00:00:FB:A4:16"}, testCfg(), testLogger())
	b := BuildEntry(config.DeviceConfig{Name: "b", MAC: "01:00:00:FB:A4:17"}, testCfg(), testLogger())

	if err := r.Add(a); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if err := r.Add(b); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if r.Len() != 2 {
		t.Errorf("Len = %d, want 2", r.Len())
	}

	// Primary = first inserted.
	if p := r.Primary(); p == nil || p.ID != a.ID {
		t.Errorf("Primary = %v, want %s", p, a.ID)
	}

	// Lookup by ID.
	got, ok := r.Get(b.ID)
	if !ok {
		t.Fatalf("Get(%s) not found", b.ID)
	}
	if got.Name != "b" {
		t.Errorf("got name = %q, want b", got.Name)
	}
}

func TestAdd_RejectsDuplicateID(t *testing.T) {
	r := New()
	a := BuildEntry(config.DeviceConfig{MAC: "01:00:00:FB:A4:16"}, testCfg(), testLogger())
	if err := r.Add(a); err != nil {
		t.Fatalf("first add: %v", err)
	}
	// Same MAC -> same ID.
	dup := BuildEntry(config.DeviceConfig{Name: "dup", MAC: "01:00:00:FB:A4:16"}, testCfg(), testLogger())
	if err := r.Add(dup); err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
}

func TestAdd_RejectsNilAndMissingID(t *testing.T) {
	r := New()
	if err := r.Add(nil); err == nil {
		t.Error("expected error for nil entry")
	}
	if err := r.Add(&Entry{}); err == nil {
		t.Error("expected error for missing ID")
	}
}

func TestPrimary_EmptyRegistry(t *testing.T) {
	r := New()
	if p := r.Primary(); p != nil {
		t.Errorf("Primary on empty registry = %v, want nil", p)
	}
}

func TestList_InsertionOrder(t *testing.T) {
	r := New()
	macs := []string{
		"01:00:00:FB:A4:16",
		"01:00:00:FB:A4:17",
		"01:00:00:FB:A4:18",
	}
	for _, m := range macs {
		if err := r.Add(BuildEntry(config.DeviceConfig{MAC: m}, testCfg(), testLogger())); err != nil {
			t.Fatalf("add %s: %v", m, err)
		}
	}
	list := r.List()
	if len(list) != 3 {
		t.Fatalf("List len = %d, want 3", len(list))
	}
	for i, e := range list {
		if e.MAC != macs[i] {
			t.Errorf("list[%d].MAC = %s, want %s", i, e.MAC, macs[i])
		}
	}
}

func TestIDs_Sorted(t *testing.T) {
	r := New()
	// Insert in non-sorted order.
	for _, m := range []string{"01:00:00:FB:A4:18", "01:00:00:FB:A4:16", "01:00:00:FB:A4:17"} {
		if err := r.Add(BuildEntry(config.DeviceConfig{MAC: m}, testCfg(), testLogger())); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	ids := r.IDs()
	want := []string{"010000fba416", "010000fba417", "010000fba418"}
	for i, id := range ids {
		if id != want[i] {
			t.Errorf("ids[%d] = %s, want %s", i, id, want[i])
		}
	}
}

func TestBuildEntry_FriendlyNameFallsBackToMAC(t *testing.T) {
	e := BuildEntry(config.DeviceConfig{MAC: "01:00:00:FB:A4:16"}, testCfg(), testLogger())
	if e.Name != "01:00:00:FB:A4:16" {
		t.Errorf("Name = %q, want MAC fallback", e.Name)
	}
}
