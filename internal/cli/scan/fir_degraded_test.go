package scan

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestFirFallbackRulesListsDefaultOnTwins(t *testing.T) {
	ids := firFallbackRules(nil)
	if len(ids) == 0 {
		t.Fatal("expected default-on rules with FIR twins")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("not sorted/unique: %v", ids)
		}
	}
}

func TestHandleFirPreflightFailure(t *testing.T) {
	cause := errors.New("no classpath")
	newFlags := func(fail bool) *scanFlags {
		noFir := false
		return &scanFlags{NoFir: &noFir, FailOnDegraded: &fail}
	}
	t.Run("warns and falls back", func(t *testing.T) {
		f := newFlags(false)
		var warn bytes.Buffer
		if err := handleFirPreflightFailure(f, nil, cause, &warn); err != nil {
			t.Fatal(err)
		}
		if !*f.NoFir || f.firDegraded == nil || !strings.Contains(warn.String(), "warning: FIR rules were skipped (no classpath)") {
			t.Fatalf("noFir=%v degraded=%v warn=%q", *f.NoFir, f.firDegraded, warn.String())
		}
	})
	t.Run("fail-on-degraded errors", func(t *testing.T) {
		f := newFlags(true)
		var warn bytes.Buffer
		err := handleFirPreflightFailure(f, nil, cause, &warn)
		if err == nil || !strings.Contains(err.Error(), "FIR rules were skipped") || warn.Len() != 0 {
			t.Fatalf("err=%v warn=%q", err, warn.String())
		}
	})
}
