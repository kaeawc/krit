package v2

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestCapabilities_Has(t *testing.T) {
	tests := []struct {
		name   string
		caps   Capabilities
		flag   Capabilities
		expect bool
	}{
		{"zero has nothing", 0, NeedsResolver, false},
		{"resolver has resolver", NeedsResolver, NeedsResolver, true},
		{"combined has resolver", NeedsResolver | NeedsCrossFile, NeedsResolver, true},
		{"combined has crossfile", NeedsResolver | NeedsCrossFile, NeedsCrossFile, true},
		{"combined missing module", NeedsResolver | NeedsCrossFile, NeedsModuleIndex, false},
		{"all bits", NeedsResolver | NeedsModuleIndex | NeedsCrossFile | NeedsLinePass, NeedsLinePass, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fix: Has checks c&flag == flag, so single bit should also work
			got := tt.caps.Has(tt.flag)
			if got != tt.expect {
				t.Errorf("Capabilities(%d).Has(%d) = %v, want %v", tt.caps, tt.flag, got, tt.expect)
			}
		})
	}
}

func TestCapabilities_IsPerFile(t *testing.T) {
	tests := []struct {
		name   string
		caps   Capabilities
		expect bool
	}{
		{"zero is per-file", 0, true},
		{"resolver is per-file", NeedsResolver, true},
		{"line pass is per-file", NeedsLinePass, true},
		{"resolver+line is per-file", NeedsResolver | NeedsLinePass, true},
		{"cross-file is not per-file", NeedsCrossFile, false},
		{"module-index is not per-file", NeedsModuleIndex, false},
		{"parsed-files is not per-file", NeedsParsedFiles, false},
		{"manifest is not per-file", NeedsManifest, false},
		{"resources is not per-file", NeedsResources, false},
		{"gradle is not per-file", NeedsGradle, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.caps.IsPerFile(); got != tt.expect {
				t.Errorf("Capabilities(%d).IsPerFile() = %v, want %v", tt.caps, got, tt.expect)
			}
		})
	}
}

func TestRule_Name(t *testing.T) {
	r := &Rule{ID: "TestRule"}
	if r.Name() != "TestRule" {
		t.Errorf("Name() = %q, want %q", r.Name(), "TestRule")
	}
}

func TestFixLevel_String(t *testing.T) {
	tests := []struct {
		level FixLevel
		want  string
	}{
		{FixNone, "none"},
		{FixCosmetic, "cosmetic"},
		{FixIdiomatic, "idiomatic"},
		{FixSemantic, "semantic"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("FixLevel(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestOracleFilter_NeverNeedsOracle(t *testing.T) {
	tests := []struct {
		name   string
		filter *OracleFilter
		want   bool
	}{
		{"nil filter", nil, false},
		{"empty filter", &OracleFilter{}, true},
		{"all-files filter", &OracleFilter{AllFiles: true}, false},
		{"identifiers filter", &OracleFilter{Identifiers: []string{"foo"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.NeverNeedsOracle(); got != tt.want {
				t.Errorf("NeverNeedsOracle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContext_Emit(t *testing.T) {
	file := &scanner.File{Path: "test.kt"}
	ctx := FakeContext(file)

	ctx.Emit(scanner.Finding{
		File:    file.Path,
		Line:    10,
		Col:     5,
		Rule:    "TestRule",
		Message: "test finding",
	})

	findings := ContextFindings(ctx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Line != 10 {
		t.Errorf("finding line = %d, want 10", findings[0].Line)
	}
	if findings[0].Message != "test finding" {
		t.Errorf("finding message = %q, want %q", findings[0].Message, "test finding")
	}
}

func TestContext_EmitAt(t *testing.T) {
	file := &scanner.File{Path: "test.kt"}
	ctx := FakeContext(file)

	ctx.EmitAt(5, 3, "issue here")

	findings := ContextFindings(ctx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.File != "test.kt" || f.Line != 5 || f.Col != 3 || f.Message != "issue here" {
		t.Errorf("unexpected finding: %+v", f)
	}
}

func TestFakeRule(t *testing.T) {
	emitted := false
	r := FakeRule("test-rule",
		WithNodeTypes("call_expression"),
		WithNeeds(NeedsResolver),
		WithFix(FixIdiomatic),
		WithConfidence(0.75),
		WithSeverity(SeverityError),
		WithOracle(&OracleFilter{Identifiers: []string{"suspend"}}),
		WithCheck(func(ctx *Context) {
			emitted = true
			ctx.EmitAt(1, 1, "found issue")
		}),
	)

	if r.ID != "test-rule" {
		t.Errorf("ID = %q, want %q", r.ID, "test-rule")
	}
	if len(r.NodeTypes) != 1 || r.NodeTypes[0] != "call_expression" {
		t.Errorf("NodeTypes = %v, want [call_expression]", r.NodeTypes)
	}
	if !r.Needs.Has(NeedsResolver) {
		t.Error("expected NeedsResolver capability")
	}
	if r.Fix != FixIdiomatic {
		t.Errorf("Fix = %v, want FixIdiomatic", r.Fix)
	}
	if r.Confidence != 0.75 {
		t.Errorf("Confidence = %f, want 0.75", r.Confidence)
	}
	if r.Sev != SeverityError {
		t.Errorf("Sev = %q, want %q", r.Sev, SeverityError)
	}

	file := &scanner.File{Path: "test.kt"}
	ctx := FakeContext(file)
	r.Check(ctx)

	if !emitted {
		t.Error("check function was not called")
	}
	if len(ContextFindings(ctx)) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(ContextFindings(ctx)))
	}
}

func TestRegister_PanicsOnEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty ID")
		}
	}()
	Register(&Rule{})
}

func TestRegister_PanicsOnNoDescription(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for no description")
		}
	}()
	Register(&Rule{ID: "test"})
}

func TestRegister_PanicsOnNoCheck(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil check")
		}
	}()
	Register(&Rule{ID: "test", Description: "desc"})
}

func TestDispatchRouting(t *testing.T) {
	// Verify the routing logic that the dispatcher will use to classify rules
	tests := []struct {
		name     string
		rule     *Rule
		isLine   bool
		isNode   bool
		isCross  bool
		isModule bool
	}{
		{
			name:   "node dispatch rule",
			rule:   FakeRule("n", WithNodeTypes("call_expression")),
			isNode: true,
		},
		{
			name:   "all-node dispatch rule",
			rule:   FakeRule("n"), // nil NodeTypes, no NeedsLinePass
			isNode: true,
		},
		{
			name:   "line rule",
			rule:   FakeRule("l", WithNeeds(NeedsLinePass)),
			isLine: true,
		},
		{
			name:    "cross-file rule",
			rule:    FakeRule("c", WithNeeds(NeedsCrossFile)),
			isCross: true,
		},
		{
			name:     "module-aware rule",
			rule:     FakeRule("m", WithNeeds(NeedsModuleIndex)),
			isModule: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.rule
			isLine := r.Needs.Has(NeedsLinePass) && r.Needs.IsPerFile()
			isNode := !isLine && r.Needs.IsPerFile()
			isCross := r.Needs.Has(NeedsCrossFile)
			isModule := r.Needs.Has(NeedsModuleIndex)

			if isLine != tt.isLine {
				t.Errorf("isLine = %v, want %v", isLine, tt.isLine)
			}
			if isNode != tt.isNode {
				t.Errorf("isNode = %v, want %v", isNode, tt.isNode)
			}
			if isCross != tt.isCross {
				t.Errorf("isCross = %v, want %v", isCross, tt.isCross)
			}
			if isModule != tt.isModule {
				t.Errorf("isModule = %v, want %v", isModule, tt.isModule)
			}
		})
	}
}

func TestMultipleFindings(t *testing.T) {
	r := FakeRule("multi",
		WithNodeTypes("call_expression"),
		WithCheck(func(ctx *Context) {
			ctx.EmitAt(1, 1, "first")
			ctx.EmitAt(2, 1, "second")
			ctx.Emit(scanner.Finding{
				File:    ctx.File.Path,
				Line:    3,
				Col:     1,
				Message: "third",
			})
		}),
	)

	file := &scanner.File{Path: "test.kt"}
	ctx := FakeContext(file)
	r.Check(ctx)

	findings := ContextFindings(ctx)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(findings))
	}
	for i, msg := range []string{"first", "second", "third"} {
		if findings[i].Message != msg {
			t.Errorf("finding[%d].Message = %q, want %q", i, findings[i].Message, msg)
		}
	}
}
