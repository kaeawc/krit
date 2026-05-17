package scanner

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

const kotlinFixtureSrc = "package fixture\n\nclass Sample { fun greet() = \"hi\" }\n"

func writeKotlinFixtures(t *testing.T, count int) []string {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, count)
	for i := 0; i < count; i++ {
		paths[i] = writeKotlin(t, dir, fmt.Sprintf("Sample%d.kt", i), kotlinFixtureSrc)
	}
	return paths
}

func TestScanFilesPreCancelledCtxSurfacesCancellation(t *testing.T) {
	paths := writeKotlinFixtures(t, 8)

	type scanFn func(context.Context, []string, int) ([]*File, []error)
	cases := []struct {
		name string
		scan scanFn
	}{
		{"ScanFiles", ScanFiles},
		{"ScanFilesCached", func(ctx context.Context, p []string, w int) ([]*File, []error) {
			return ScanFilesCached(ctx, p, w, nil)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			files, errs := tc.scan(ctx, paths, 4)

			if len(files) != 0 {
				t.Fatalf("expected zero files for pre-cancelled ctx, got %d", len(files))
			}
			if got := countErrs(errs, context.Canceled); got != 1 {
				t.Fatalf("expected exactly one context.Canceled error, got %d (errs=%v)", got, errs)
			}
		})
	}
}

func TestScanFilesDeadlineSurfacesDeadlineExceeded(t *testing.T) {
	paths := writeKotlinFixtures(t, 8)

	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()

	files, errs := ScanFiles(ctx, paths, 4)

	if len(files) != 0 {
		t.Fatalf("expected zero files for expired deadline, got %d", len(files))
	}
	if !errsContain(errs, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded in errs, got %v", errs)
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

func countErrs(errs []error, target error) int {
	n := 0
	for _, e := range errs {
		if errors.Is(e, target) {
			n++
		}
	}
	return n
}

func errsContain(errs []error, target error) bool {
	return countErrs(errs, target) > 0
}
