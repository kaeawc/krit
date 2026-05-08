package ruleslinter

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestRulesSubpackageTopology gates the per-domain dependency contract:
// every Go subpackage under internal/rules/ may import only the
// foundation tier (scanner, v2, base, typeinfer, oracle, analyzers/*,
// and a small set of internal data packages). Importing the parent
// rules package or a sibling domain subpackage is forbidden.
//
// A new rule subpackage that reaches across the topology fails this
// test.
func TestRulesSubpackageTopology(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	rulesDir := filepath.Join(filepath.Dir(thisFile), "..", "rules")
	violations, err := AnalyzeSubpackageTopology(rulesDir)
	if err != nil {
		t.Fatalf("AnalyzeSubpackageTopology: %v", err)
	}
	if len(violations) == 0 {
		return
	}
	var b strings.Builder
	for _, v := range violations {
		b.WriteString(v.String())
		b.WriteByte('\n')
	}
	t.Fatalf("ruleslinter found %d topology violation(s):\n%s", len(violations), b.String())
}

func TestClassifyImport_AllowsFoundation(t *testing.T) {
	cases := []string{
		"github.com/kaeawc/krit/internal/scanner",
		"github.com/kaeawc/krit/internal/rules/api",
		"github.com/kaeawc/krit/internal/rules/base",
		"github.com/kaeawc/krit/internal/rules/semantics",
		"github.com/kaeawc/krit/internal/analyzers/nullflow",
		"github.com/kaeawc/krit/internal/analyzers/astflat",
		"github.com/kaeawc/krit/internal/typeinfer",
		"strings",
		"go/ast",
	}
	for _, c := range cases {
		if reason, forbidden := classifyImport(c, "coroutines"); forbidden {
			t.Errorf("classifyImport(%q) forbidden=%v reason=%q; want allowed", c, forbidden, reason)
		}
	}
}

func TestClassifyImport_ForbidsParentRules(t *testing.T) {
	reason, forbidden := classifyImport("github.com/kaeawc/krit/internal/rules", "coroutines")
	if !forbidden {
		t.Fatalf("classifyImport(parent rules) forbidden=false; want true")
	}
	if !strings.Contains(reason, "parent rules package") {
		t.Errorf("reason=%q; want mention of parent rules package", reason)
	}
}

func TestClassifyImport_ForbidsSiblingDomain(t *testing.T) {
	reason, forbidden := classifyImport("github.com/kaeawc/krit/internal/rules/database", "coroutines")
	if !forbidden {
		t.Fatalf("classifyImport(sibling) forbidden=false; want true")
	}
	if !strings.Contains(reason, "sibling subpackage") {
		t.Errorf("reason=%q; want mention of sibling subpackage", reason)
	}
}

func TestClassifyImport_AllowsSelfDomain(t *testing.T) {
	if reason, forbidden := classifyImport("github.com/kaeawc/krit/internal/rules/coroutines", "coroutines"); forbidden {
		t.Errorf("classifyImport(self domain) forbidden=true reason=%q; want allowed", reason)
	}
}
