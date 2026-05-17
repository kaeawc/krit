package scanner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeKotlinFixtures(t *testing.T, count int) []string {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, count)
	src := []byte("package fixture\n\nclass Sample { fun greet() = \"hi\" }\n")
	for i := 0; i < count; i++ {
		p := filepath.Join(dir, fmt.Sprintf("Sample%d.kt", i))
		if err := os.WriteFile(p, src, 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		paths[i] = p
	}
	return paths
}

func TestScanFilesPreCancelledCtxStopsShort(t *testing.T) {
	paths := writeKotlinFixtures(t, 16)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	files, errs := ScanFiles(ctx, paths, 4)

	if len(files) != 0 {
		t.Fatalf("expected zero files parsed for pre-cancelled ctx, got %d", len(files))
	}
	if !errsContain(errs, context.Canceled) {
		t.Fatalf("expected context.Canceled in errs, got %v", errs)
	}
}

func TestScanFilesPreCancelledCtxReturnsCancelErr(t *testing.T) {
	paths := writeKotlinFixtures(t, 8)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, errs := ScanFilesCached(ctx, paths, 2, nil)

	if !errsContain(errs, context.Canceled) {
		t.Fatalf("expected context.Canceled in errs, got %v", errs)
	}
}

func TestScanFilesDeadlineSurfacesError(t *testing.T) {
	paths := writeKotlinFixtures(t, 64)

	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()

	files, errs := ScanFiles(ctx, paths, 4)

	if len(files) != 0 {
		t.Fatalf("expected zero files for expired deadline, got %d", len(files))
	}
	if !errsContainAny(errs, context.DeadlineExceeded, context.Canceled) {
		t.Fatalf("expected DeadlineExceeded or Canceled in errs, got %v", errs)
	}
}

func TestScanFilesSucceedsWithLiveCtx(t *testing.T) {
	paths := writeKotlinFixtures(t, 4)

	files, errs := ScanFiles(t.Context(), paths, 2)

	if len(files) != len(paths) {
		t.Fatalf("expected %d files, got %d (errs: %v)", len(paths), len(files), errs)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no errs for live ctx, got %v", errs)
	}
}

func errsContain(errs []error, target error) bool {
	for _, e := range errs {
		if errors.Is(e, target) {
			return true
		}
	}
	return false
}

func errsContainAny(errs []error, targets ...error) bool {
	for _, t := range targets {
		if errsContain(errs, t) {
			return true
		}
	}
	return false
}
