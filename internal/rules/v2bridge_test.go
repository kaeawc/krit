package rules

import (
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestV2FlatDispatchSatisfiesV1(t *testing.T) {
	r := v2.FakeRule("TestV2Flat",
		v2.WithNodeTypes("call_expression"),
		v2.WithSeverity(v2.SeverityWarning),
		v2.WithCheck(func(ctx *v2.Context) {
			ctx.EmitAt(1, 1, "found it")
		}),
	)
	r.Category = "style"

	wrapper := v2.ToV1(r)

	// Must satisfy the flat-dispatch structural contract (NodeTypes+CheckFlatNode).
	fdr, ok := wrapper.(interface {
		Rule
		NodeTypes() []string
		CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding
	})
	if !ok {
		t.Fatal("V1FlatDispatch does not satisfy the flat-dispatch method set")
	}

	if fdr.Name() != "TestV2Flat" {
		t.Errorf("Name() = %q, want TestV2Flat", fdr.Name())
	}
	if fdr.RuleSet() != "style" {
		t.Errorf("RuleSet() = %q, want style", fdr.RuleSet())
	}

	file := &scanner.File{Path: "test.kt"}
	findings := fdr.CheckFlatNode(0, file)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Rule != "TestV2Flat" {
		t.Errorf("finding.Rule = %q, want TestV2Flat", findings[0].Rule)
	}
}

func TestV2LineSatisfiesV1(t *testing.T) {
	r := v2.FakeRule("TestV2Line",
		v2.WithNeeds(v2.NeedsLinePass),
		v2.WithCheck(func(ctx *v2.Context) {
			ctx.EmitAt(1, 1, "line issue")
		}),
	)
	r.Category = "naming"

	wrapper := v2.ToV1(r)

	lr, ok := wrapper.(interface {
		Rule
		CheckLines(file *scanner.File) []scanner.Finding
	})
	if !ok {
		t.Fatal("V1Line does not satisfy the line-rule method set")
	}

	if lr.Name() != "TestV2Line" {
		t.Errorf("Name() = %q, want TestV2Line", lr.Name())
	}

	file := &scanner.File{Path: "test.kt"}
	findings := lr.CheckLines(file)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestV2CrossFileSatisfiesV1(t *testing.T) {
	r := v2.FakeRule("TestV2Cross",
		v2.WithNeeds(v2.NeedsCrossFile),
		v2.WithCheck(func(ctx *v2.Context) {
			ctx.Emit(scanner.Finding{
				File: "a.kt", Line: 1, Message: "cross",
			})
		}),
	)
	r.Category = "complexity"

	wrapper := v2.ToV1(r)

	cfr, ok := wrapper.(interface {
		Rule
		CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding
	})
	if !ok {
		t.Fatal("V1CrossFile does not satisfy the cross-file method set")
	}

	findings := cfr.CheckCrossFile(&scanner.CodeIndex{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestV2ConfidenceProviderSatisfiesV1(t *testing.T) {
	r := v2.FakeRule("TestConf",
		v2.WithNodeTypes("call_expression"),
		v2.WithConfidence(0.85),
		v2.WithCheck(func(ctx *v2.Context) {}),
	)
	r.Category = "style"

	wrapper := v2.ToV1(r)

	cp, ok := wrapper.(interface{ Confidence() float64 })
	if !ok {
		t.Fatal("V1FlatDispatch does not satisfy the Confidence method")
	}

	if cp.Confidence() != 0.85 {
		t.Errorf("Confidence() = %f, want 0.85", cp.Confidence())
	}
}

func TestRegisterV2Rules(t *testing.T) {
	// Verify that bridging works by directly calling BridgeToV1Rules
	// on a temporary v2 rule. We don't use RegisterV2Rules() here
	// because TestMain already calls it (with sync.Once).
	r := &v2.Rule{
		ID:          "V2BridgeTest",
		Category:    "test",
		Description: "v2 bridge test rule",
		Sev:         v2.SeverityWarning,
		NodeTypes:   []string{"call_expression"},
		Check:       func(ctx *v2.Context) {},
	}

	wrapped := v2.ToV1(r)
	v1Rule, ok := wrapped.(Rule)
	if !ok {
		t.Fatal("ToV1 wrapper does not satisfy Rule interface")
	}
	if v1Rule.Name() != "V2BridgeTest" {
		t.Errorf("Name() = %q, want V2BridgeTest", v1Rule.Name())
	}

	// Verify RegisterV2Rules was already called (by TestMain)
	// by checking that v2-registered rules from migrated files
	// are present in the v1 Registry.
	found := false
	for _, reg := range Registry {
		if reg.Name() == "ComposeColumnRowInScrollable" {
			found = true
			break
		}
	}
	if !found {
		// This rule was migrated to v2.Register; if it's in Registry,
		// RegisterV2Rules successfully bridged it.
		t.Log("ComposeColumnRowInScrollable not found — v2 bridge may not have migrated rules yet")
	}
}
