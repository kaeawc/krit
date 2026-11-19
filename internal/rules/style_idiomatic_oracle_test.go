package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// The operand's type decides check(x != null) and require(x != null). Source inference cannot type a local read
// from an unresolved class, and the oracle's type of the whole call is Unit, which says nothing about the operand.
func TestNonNullCheckRules_UseOperandTypeFromOracle(t *testing.T) {
	for _, tc := range []struct{ rule, fn string }{{"UseCheckNotNull", "check"}, {"UseRequireNotNull", "require"}} {
		file := parseInline(t, "import com.example.Msg\nfun f(m: Msg) {\n    val ts = m.threadTs\n    "+tc.fn+"(ts != null)\n}\n")
		call, ok := findFlatNodeOf(t, file, "call_expression", tc.fn+"(ts != null)")
		if !ok {
			t.Fatal("could not locate the call")
		}
		var operand uint32
		file.FlatWalkNodes(call, "simple_identifier", func(idx uint32) {
			if file.FlatNodeText(idx) == "ts" {
				operand = idx
			}
		})
		if operand == 0 {
			t.Fatal("could not locate the operand")
		}
		fact := func(node uint32, typ string, nullable bool) *oracle.ExpressionType {
			return &oracle.ExpressionType{Type: typ, Nullable: nullable,
				StartByte: int(file.FlatStartByte(node)), EndByte: int(file.FlatEndByte(node))}
		}
		o, err := oracle.LoadFromData(&oracle.Data{
			Version: 1, KotlinVersion: "2.1.0",
			Files: map[string]*oracle.File{file.Path: {Expressions: map[string]*oracle.ExpressionType{
				positionKey(file, call):    fact(call, "kotlin.Unit", false),
				positionKey(file, operand): fact(operand, "kotlin.String", true),
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
			if r.ID == tc.rule {
				rule = r
			}
		}
		cols := rules.NewDispatcher([]*api.Rule{rule}, oracle.NewCompositeResolver(o, resolver)).Run(file)
		findings := cols.Findings()
		if len(findings) != 1 || findings[0].Line != 4 {
			t.Errorf("%s: expected one finding on line 4, got %v", tc.rule, findings)
		}
	}
}

// runWithOracleFacts runs rule on code with the oracle facts that facts returns for the parsed file.
func runWithOracleFacts(t *testing.T, ruleID, code string, facts func(*scanner.File) map[string]*oracle.ExpressionType) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	o, err := oracle.LoadFromData(&oracle.Data{
		Version: 1, KotlinVersion: "2.1.0",
		Files:        map[string]*oracle.File{file.Path: {Expressions: facts(file)}},
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
	cols := rules.NewDispatcher([]*api.Rule{rule}, oracle.NewCompositeResolver(o, resolver)).Run(file)
	return cols.Findings()
}

// oracleFactFor returns a fact for the first node of kind whose text is text.
func oracleFactFor(t *testing.T, file *scanner.File, kind, text, typ string, nullable bool) (string, *oracle.ExpressionType) {
	t.Helper()
	node, ok := findFlatNodeOf(t, file, kind, text)
	if !ok {
		t.Fatalf("no %s %q", kind, text)
	}
	return positionKey(file, node), &oracle.ExpressionType{Type: typ, Nullable: nullable,
		StartByte: int(file.FlatStartByte(node)), EndByte: int(file.FlatEndByte(node))}
}

// A platform type such as `String!` can hold null, so check(x != null) on it is still a not-null check, and
// detekt's isString accepts it as a message.
func TestStdlibIdiomRules_PlatformTypeFromOracle(t *testing.T) {
	getenv := func(file *scanner.File) map[string]*oracle.ExpressionType {
		k, v := oracleFactFor(t, file, "call_expression", `System.getenv("X")`, "String!", false)
		return map[string]*oracle.ExpressionType{k: v}
	}
	cases := []struct{ rule, code string }{
		{"UseCheckNotNull", "fun f() {\n    check(System.getenv(\"X\") != null)\n}\n"},
		{"UseRequireNotNull", "fun f() {\n    require(System.getenv(\"X\") != null)\n}\n"},
		{"UseCheckOrError", "fun f() {\n    throw IllegalStateException(System.getenv(\"X\"))\n}\n"},
		{"UseRequire", "fun f(c: Boolean) {\n    if (c) throw IllegalArgumentException(System.getenv(\"X\"))\n}\n"},
	}
	for _, c := range cases {
		if got := runWithOracleFacts(t, c.rule, c.code, getenv); len(got) != 1 || got[0].Line != 2 {
			t.Errorf("%s: expected one finding on line 2, got %v", c.rule, got)
		}
	}
}

// `x!!` is non-null even when the oracle only recorded the nullable operand.
func TestUseCheckOrError_NotNullAssertedMessageFromOracle(t *testing.T) {
	got := runWithOracleFacts(t, "UseCheckOrError", "class H(val message: String?)\nfun f(h: H) {\n    throw IllegalStateException(h.message!!)\n}\n",
		func(file *scanner.File) map[string]*oracle.ExpressionType {
			k, v := oracleFactFor(t, file, "navigation_expression", "h.message", "kotlin.String?", true)
			return map[string]*oracle.ExpressionType{k: v}
		})
	if len(got) != 1 || got[0].Line != 3 {
		t.Fatalf("expected one finding on line 3, got %v", got)
	}
}
