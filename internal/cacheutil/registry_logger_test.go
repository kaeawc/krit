package cacheutil

import (
	"log/slog"
	"testing"

	"github.com/kaeawc/krit/internal/logger"
)

type fakeRegistered struct{ name string }

func (f fakeRegistered) Name() string                 { return f.name }
func (f fakeRegistered) Clear(ctx ClearContext) error { return nil }

// TestRegisterDuplicateNameLogsViaPkgLog verifies that registering a
// second cache with a name already in the registry routes the warning
// through the package Logger (not the standard log package), so tests
// can observe the record via logger.NewCapture.
func TestRegisterDuplicateNameLogsViaPkgLog(t *testing.T) {
	prev := pkgLog
	cap := logger.NewCapture(slog.LevelDebug)
	SetLogger(cap)
	t.Cleanup(func() { SetLogger(prev) })

	// Reset the registry so this test doesn't depend on init order.
	mu.Lock()
	prevReg := registry
	registry = nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		registry = prevReg
		mu.Unlock()
	})

	Register(fakeRegistered{name: "dup"})
	Register(fakeRegistered{name: "dup"}) // triggers the warning path

	if !cap.HasMessage("replacing already-registered cache") {
		t.Fatalf("expected duplicate-name warning record, got %+v", cap.Records())
	}
	warns := cap.FilterLevel(slog.LevelWarn)
	if len(warns) != 1 {
		t.Errorf("expected exactly 1 Warn record, got %d", len(warns))
	}
	if name, ok := warns[0].Attrs["name"]; !ok || name != "dup" {
		t.Errorf("expected name=dup attr on warn record, got attrs=%v", warns[0].Attrs)
	}
}
