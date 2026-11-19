package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// runNonNullCheckWithOracle runs ruleID with compiler facts that type the call itself as kotlin.Unit and the
// operand (the first `operandText` in the call) as operandType.
func runNonNullCheckWithOracle(t *testing.T, ruleID, code, operandText, operandType string, operandNullable bool) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	var call, operand uint32
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if call == 0 {
			call = idx
		}
	})
	file.FlatWalkAllNodes(call, func(idx uint32) {
		if operand == 0 && file.FlatType(idx) != "value_argument" && strings.TrimSpace(file.FlatNodeText(idx)) == operandText {
			operand = idx
		}
	})
	if call == 0 || operand == 0 {
		t.Fatalf("call %d or operand %q not found", call, operandText)
	}
	fact := func(idx uint32, typ string, nullable bool) *oracle.ExpressionType {
		return &oracle.ExpressionType{Type: typ, Nullable: nullable, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx))}
	}
	o, err := oracle.LoadFromData(&oracle.Data{
		Version:       1,
		KotlinVersion: "2.1.0",
		Files: map[string]*oracle.File{file.Path: {Expressions: map[string]*oracle.ExpressionType{
			"1:1": fact(call, "kotlin.Unit", false),
			"1:2": fact(operand, operandType, operandNullable),
		}}},
		Dependencies: map[string]*oracle.Class{},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == ruleID {
			rule = r
		}
	}
	if rule == nil {
		t.Fatalf("%s not registered", ruleID)
	}
	cols := rules.NewDispatcher([]*api.Rule{rule}, oracle.NewCompositeResolver(o, resolver)).Run(file)
	return cols.Findings()
}

// With compiler facts the call's own position holds its type, kotlin.Unit; the operand's type decides.
func TestNonNullCheck_OracleReadsOperandNotCall(t *testing.T) {
	code := "package test\n\nfun f(h: Holder) {\n    require(h.value != null)\n}\n"
	if got := runNonNullCheckWithOracle(t, "UseRequireNotNull", code, "h.value", "kotlin.String?", true); len(got) != 1 {
		t.Fatalf("nullable operand: want 1 finding, got %v", got)
	}
	if got := runNonNullCheckWithOracle(t, "UseRequireNotNull", code, "h.value", "kotlin.String", false); len(got) != 0 {
		t.Fatalf("non-null operand: want no finding, got %v", got)
	}
	// A Java getter's platform type can hold null; the facts mark it non-null and spell it with a `!`.
	if got := runNonNullCheckWithOracle(t, "UseRequireNotNull", code, "h.value", "String!", false); len(got) != 1 {
		t.Fatalf("platform operand: want 1 finding, got %v", got)
	}
	checkCode := strings.Replace(code, "require(", "check(", 1)
	if got := runNonNullCheckWithOracle(t, "UseCheckNotNull", checkCode, "h.value", "kotlin.String?", true); len(got) != 1 {
		t.Fatalf("UseCheckNotNull nullable operand: want 1 finding, got %v", got)
	}
}

func TestUseRequireNotNull_SkipsLookalikesTheFileShows(t *testing.T) {
	cases := map[string]string{
		"explicit receiver": "fun f(v: Validator, x: Any?) { v.require(x != null) }",
		"object member":     "object Obj { fun require(b: Boolean) = Unit }\nfun f(x: Any?) { Obj.require(x != null) }",
		"class member":      "class V {\n    fun require(b: Boolean) = Unit\n    fun f(x: Any?) { require(x != null) }\n}",
		"top-level":         "fun require(b: Boolean) = Unit\nfun f(x: Any?) { require(x != null) }",
		"local function":    "fun f(x: Any?) {\n    fun require(b: Boolean) = Unit\n    require(x != null)\n}",
		"local value":       "fun f(x: Any?) {\n    val require: (Boolean) -> Unit = {}\n    require(x != null)\n}",
		"explicit import":   "import other.Guards.require\nfun f(x: Any?) { require(x != null) }",
		"aliased stdlib":    "import kotlin.require as ensure\nfun f(x: Any?) { require(x != null) }",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if got := runRuleByName(t, "UseRequireNotNull", "package test\n"+body+"\n"); len(got) != 0 {
				t.Fatalf("want no finding, got %v", got)
			}
		})
	}
}

func TestUseRequireNotNull_ReportsStdlibCallsBesideLookalikes(t *testing.T) {
	cases := map[string]string{
		"kotlin qualifier":        "fun f(x: Any?) { kotlin.require(x != null) }",
		"stdlib import":           "import kotlin.require\nfun f(x: Any?) { require(x != null) }",
		"extension elsewhere":     "class T\nfun T.require(b: Boolean) = Unit\nfun f(x: Any?) { require(x != null) }",
		"member of another class": "class V { fun require(b: Boolean) = Unit }\nfun f(x: Any?) { require(x != null) }",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if got := runRuleByName(t, "UseRequireNotNull", "package test\n"+body+"\n"); len(got) != 1 {
				t.Fatalf("want 1 finding, got %v", got)
			}
		})
	}
}

// detekt reads only the first argument, so a lazy message inside the parentheses still reports, and the fix keeps it.
func TestNonNullCheck_MessageInsideParentheses(t *testing.T) {
	cases := []struct{ rule, code, fix string }{
		{"UseRequireNotNull", `require(x != null, { "m" })`, `requireNotNull(x, { "m" })`},
		{"UseRequireNotNull", `require(value = null != x, lazyMessage = { "m" })`, `requireNotNull(x, lazyMessage = { "m" })`},
		{"UseCheckNotNull", `check(x != null, { "m" })`, `checkNotNull(x, { "m" })`},
		// The trailing lambda stays in place: the fix rewrites the call before it.
		{"UseRequireNotNull", `require(x != null) { "m" }`, `requireNotNull(x)`},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			got := runRuleByName(t, tc.rule, "package test\nfun f(x: Any?) {\n    "+tc.code+"\n}\n")
			if len(got) != 1 || got[0].Fix == nil || got[0].Fix.Replacement != tc.fix {
				if len(got) == 1 && got[0].Fix != nil {
					t.Fatalf("want fix %q, got %q", tc.fix, got[0].Fix.Replacement)
				}
				t.Fatalf("want 1 finding with fix %q, got %+v", tc.fix, got)
			}
		})
	}
	if got := runRuleByName(t, "UseRequireNotNull", "package test\nfun f(x: Any?) {\n    require(lazyMessage = { \"m\" }, value = x != null)\n}\n"); len(got) != 0 {
		t.Fatalf("message first: want no finding, got %v", got)
	}
}
