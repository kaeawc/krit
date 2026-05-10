package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestOracleFilterFingerprint_StableAndDistinct verifies the
// fingerprint is order-independent over rules + files and changes
// when either input changes.
func TestOracleFilterFingerprint_StableAndDistinct(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "A.kt")
	b := filepath.Join(dir, "B.kt")
	if err := os.WriteFile(a, []byte("package x\nclass A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("package x\nclass B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pa, _ := scanner.ParseFile(a)
	pb, _ := scanner.ParseFile(b)

	r1 := api.FakeRule("R1")
	r2 := api.FakeRule("R2")

	fp1 := oracleFilterFingerprint([]*api.Rule{r1, r2}, []*scanner.File{pa, pb})
	fp2 := oracleFilterFingerprint([]*api.Rule{r2, r1}, []*scanner.File{pb, pa})
	if fp1 != fp2 {
		t.Errorf("fingerprint should be order-independent: %q vs %q", fp1, fp2)
	}

	// Different rule set → different fingerprint.
	fp3 := oracleFilterFingerprint([]*api.Rule{r1}, []*scanner.File{pa, pb})
	if fp1 == fp3 {
		t.Errorf("fingerprint should differ for distinct rule sets")
	}

	// Different file set → different fingerprint.
	fp4 := oracleFilterFingerprint([]*api.Rule{r1, r2}, []*scanner.File{pa})
	if fp1 == fp4 {
		t.Errorf("fingerprint should differ for distinct file sets")
	}
}

// TestSelectOracleCallFilter_PrefersPrebuilt confirms that when
// IndexInput.PrebuiltOracleCallFilter is supplied, it is returned
// without invoking the per-call buildOracleCallTargetFilterForInvocation
// path. Asserts via a non-zero CalleeNames marker.
func TestSelectOracleCallFilter_PrefersPrebuilt(t *testing.T) {
	prebuilt := &oracle.CallTargetFilterSummary{
		Enabled:     true,
		CalleeNames: []string{"sentinel.from.cache"},
	}
	in := IndexInput{PrebuiltOracleCallFilter: prebuilt}

	// loadFiles intentionally returns nil — if selectOracleCallFilter
	// fell through to the build path it would error or produce a
	// different summary.
	got := selectOracleCallFilter(in, nil, nil, nil)
	if got != prebuilt {
		t.Fatalf("selectOracleCallFilter returned %#v; want the prebuilt %#v", got, prebuilt)
	}
}

// TestSelectOracleCallFilter_PrebuiltDisabledReturnsNil verifies that
// a prebuilt summary with Enabled=false acts as a sentinel: the
// caller gets nil, matching the per-call path's "filter classified as
// disabled by broad rules" output. Without this symmetry, a cached
// "disabled" classification would unexpectedly enable the JVM-side
// gate.
func TestSelectOracleCallFilter_PrebuiltDisabledReturnsNil(t *testing.T) {
	prebuilt := &oracle.CallTargetFilterSummary{
		Enabled:    false,
		DisabledBy: []string{"BroadRule"},
	}
	in := IndexInput{PrebuiltOracleCallFilter: prebuilt}

	got := selectOracleCallFilter(in, nil, nil, nil)
	if got != nil {
		t.Fatalf("selectOracleCallFilter returned %#v; want nil for disabled prebuilt", got)
	}
}
