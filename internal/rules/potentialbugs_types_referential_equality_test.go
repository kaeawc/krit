package rules_test

import (
	"fmt"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestAvoidReferentialEquality_CompositeBindingEvidence(t *testing.T) {
	for _, testCase := range []struct {
		name string
		fqn  string
		want int
	}{
		{"source fallback", "", 0},
		{"oracle Any", "kotlin.Any", 0},
		{"oracle String", "kotlin.String", 1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			file := parseInline(t, `val label = "x"
open class Base { val label: Any = Any() }
class Holder : Base() {
    fun same(candidate: Any) = label === candidate
}`)
			resolver := typeinfer.NewResolver()
			resolver.IndexFilesParallel([]*scanner.File{file}, 1)
			fake := oracle.NewFakeOracle()
			fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{}
			if testCase.fqn != "" {
				file.FlatWalkNodes(0, "equality_expression", func(expression uint32) {
					operand := file.FlatFirstChild(expression)
					key := fmt.Sprintf("%d:%d", file.FlatRow(operand)+1, file.FlatCol(operand)+1)
					fake.Expressions[file.Path][key] = &typeinfer.ResolvedType{FQN: testCase.fqn, Kind: typeinfer.TypeClass, Resolved: true}
				})
			}
			for _, rule := range api.Registry {
				if rule.ID == "AvoidReferentialEquality" {
					columns := rules.NewDispatcher([]*api.Rule{rule}, oracle.NewCompositeResolver(fake, resolver)).Run(file)
					findings := columns.Findings()
					if len(findings) != testCase.want {
						t.Fatalf("got %d findings, want %d: %v", len(findings), testCase.want, findings)
					}
					return
				}
			}
			t.Fatal("AvoidReferentialEquality is not registered")
		})
	}
}

func TestAvoidReferentialEquality_VarargArrayTypes(t *testing.T) {
	testCases := []struct {
		name, parameterType, resolvedFQN, pattern string
		want                                      int
	}{
		{"primitive element maps to primitive array", "Int", "kotlin.Int", "kotlin.IntArray", 1},
		{"primitive element does not map to generic array", "Int", "kotlin.Int", "kotlin.Array", 0},
		{"resolved primitive array is preserved", "Int", "kotlin.IntArray", "kotlin.IntArray", 1},
		{"String element maps to generic array", "String", "kotlin.String", "kotlin.Array", 1},
		{"String vararg does not match String", "String", "kotlin.String", "kotlin.String", 0},
	}
	var rule *rules.AvoidReferentialEqualityRule
	for _, candidate := range api.Registry {
		if candidate.ID == "AvoidReferentialEquality" {
			rule, _ = candidate.Implementation.(*rules.AvoidReferentialEqualityRule)
			break
		}
	}
	if rule == nil {
		t.Fatal("AvoidReferentialEquality rule not registered")
	}
	originalPatterns := rule.ForbiddenTypePatterns
	defer func() { rule.ForbiddenTypePatterns = originalPatterns }()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			rule.ForbiddenTypePatterns = []string{testCase.pattern}
			file := parseInline(t, fmt.Sprintf("fun same(vararg values: %s, other: Any) = values === other", testCase.parameterType))
			varargOperand := uint32(0)
			file.FlatWalkNodes(0, "equality_expression", func(expression uint32) {
				varargOperand = file.FlatFirstChild(expression)
			})
			if varargOperand == 0 {
				t.Fatal("equality expression has no left operand")
			}
			fake := oracle.NewFakeOracle()
			fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{}
			key := fmt.Sprintf("%d:%d", file.FlatRow(varargOperand)+1, file.FlatCol(varargOperand)+1)
			fake.Expressions[file.Path][key] = &typeinfer.ResolvedType{FQN: testCase.resolvedFQN, Kind: typeinfer.TypeClass, Resolved: true}
			findings := runRuleOnFileWithFakeOracle(t, "AvoidReferentialEquality", file, fake)
			if len(findings) != testCase.want {
				t.Fatalf("got %d findings, want %d: %v", len(findings), testCase.want, findings)
			}
		})
	}
}

func TestAvoidReferentialEquality_ResolverlessLiteralsOnly(t *testing.T) {
	testCases := []struct {
		name string
		code string
		want int
	}{
		{"unresolved identifiers", `fun same() = unwrapped !== current`, 0},
		{"typed identifiers", `fun same(text: String, other: String) = text === other`, 0},
		{"literal initialized identifier", `val text = "a"; fun same(other: Any) = text === other`, 0},
		{"literal", `fun same(other: Any) = other === "a"`, 1},
		{"parenthesized literal", `fun same(other: Any) = other === ("a")`, 1},
		{"template", `fun same(other: Any) = "$other" !== other`, 1},
		{"raw template", "fun same(other: Any) = \"\"\"$other\"\"\" !== other", 1},
		{"literal despite lookalike", `class String; fun same(other: Any) = "a" === other`, 1},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			findings := runRuleByName(t, "AvoidReferentialEquality", testCase.code)
			if len(findings) != testCase.want {
				t.Fatalf("got %d findings, want %d: %v", len(findings), testCase.want, findings)
			}
		})
	}
}

func TestAvoidReferentialEquality_ResolverEvidence(t *testing.T) {
	testCases := []struct {
		name string
		code string
		want int
	}{
		{"wildcard import", "import kotlinx.coroutines.*\nfun same(a: String, b: String) = a === b", 1},
		{"builtin self alias", "import kotlin.String as String\nfun same(a: String, b: String) = a === b", 1},
		{"explicit lookalike import", "import example.String\nfun same(a: String, b: String) = a === b", 0},
		{"aliased lookalike import", "import example.Text as String\nfun same(a: String, b: String) = a === b", 0},
		{"same file lookalike", `class String; fun same(a: String, b: String) = a === b`, 0},
		{"vararg", `fun same(vararg texts: String, other: Any) = texts === other`, 0},
		{"vararg right", `fun same(other: Any, vararg texts: String) = other === texts`, 0},
		{"captured vararg", `fun same(vararg texts: String, other: Any) = run { texts === other }`, 0},
		{"plain parameter after vararg", `fun same(vararg texts: String, text: String, other: Any) = text === other`, 1},
		{"shadowed vararg", `fun same(vararg text: String, other: Any) { run { val text: String = "a"; text === other } }`, 1},
		{"nested function shadows vararg", `fun outer(vararg text: String) { fun same(text: String, other: Any) = text === other }`, 1},
		{"constructor vararg unresolved", `class Tags { val tags: List<String> = emptyList(); constructor(vararg tags: String, other: Any) { tags === other } }`, 0},
		{"resolved class lambda capture", `class Holder { fun same(a: String, b: String) = run { a === b } }`, 1},
		{"resolved top level inside class", `val label = "x"; class Holder { fun same(other: Any) = label === other }`, 1},
		{"inherited getter", `val label = "x"
fun fetch(): Any = Any()
open class Base { val label: Any = Any() }
class Holder : Base() {
    val same get() = label === fetch()
}`, 0},
		{"inherited object literal", `val label = "x"; open class Base { val label: Any = Any() }; fun make(candidate: Any) = object : Base() { fun same() = label === candidate }`, 0},
		{"extension function", `val label = "x"; class Node { val label: Any = Any() }; fun Node.same(candidate: Any) = label === candidate`, 0},
		{"extension property", `val label = "x"; fun fetch(): Any = Any(); class Node { val label: Any = Any() }; val Node.same: Boolean get() = label === fetch()`, 0},
		{"inherited lambda capture", `val a = "x"; open class Base { val a: Any = Any() }; class Holder : Base() { fun same(a: String, b: String) = run { a === b } }`, 1},
		{"inherited local", `val label = "x"; open class Base; class Holder : Base() { fun same(candidate: Any): Boolean { val label: String = "a"; return label === candidate } }`, 1},
		{"extension parameter", `val label = "x"; class Node; fun Node.same(label: String, candidate: Any) = label === candidate`, 1},
		{"inherited literal", `val label = "x"; open class Base; class Holder : Base() { fun same() = label === "a" }`, 1},
		{"parenthesized null", `fun same(text: String?) = text === (null)`, 0},
		{"nested parenthesized null", `fun same(text: String?) = ((null)) !== text`, 0},
		{"extension right operand", `val label = "x"; class Node { val label: Any = Any() }; fun Node.same(candidate: Any) = candidate === label`, 0},
		{"safe receiver chain", `fun normalizeOrNull(text: String): String? = text.trim(); fun same(text: String) = normalizeOrNull(text)?.trim() === text`, 0},
		{"nested safe receiver chain", `class Normalizer { fun normalize(text: String): String = text.trim() }; fun same(holder: Normalizer?, text: String) = holder?.normalize(text)?.trim() === text`, 0},
		{"receiver chain", `fun normalize(text: String) = text.trim(); fun same(text: String) = normalize(text).trim() === text`, 0},
		{"builder receiver chain", `fun same(text: String) = StringBuilder().append(text).toString() === text`, 0},
		{"parenthesized call", `fun normalize(text: String) = text.trim(); fun same(text: String) = (normalize(text)) === text`, 1},
		{"not null call", `fun normalize(text: String): String? = text; fun same(text: String) = normalize(text)!! === text`, 1},
		{"alias type parameter resolver limitation", `typealias Text = String; fun <Text> same(a: Text, b: Any) = a === b`, 1},
		{"companion unresolved identifier", `class Row(id: String) { companion object : Entity("x") { val same = id === fetch() } }`, 0},
		{"unresolved annotated identifier with resolved Any", `fun same(other: Any) { val text: kotlin.String? = "a"; text === other }`, 0},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			findings := runRuleByNameWithResolver(t, "AvoidReferentialEquality", testCase.code)
			if len(findings) != testCase.want {
				t.Fatalf("got %d findings, want %d: %v", len(findings), testCase.want, findings)
			}
		})
	}
}

func TestAvoidReferentialEquality_EnumClassBodyShadowing(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "AvoidReferentialEquality", `val label = "x"
enum class Mode { A; val label: Any = Any(); fun same(other: Any) = label === other }`)
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want none: %v", len(findings), findings)
	}
}
