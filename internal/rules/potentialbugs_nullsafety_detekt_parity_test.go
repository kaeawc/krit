package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func bangBangRuleLines(findings []scanner.Finding, rule string) []int {
	var lines []int
	for _, f := range findings {
		if f.Rule == rule {
			lines = append(lines, f.Line)
		}
	}
	return lines
}

func runDispatcherFindings(rule *api.Rule, resolver typeinfer.TypeResolver, file *scanner.File) []scanner.Finding {
	cols := rules.NewDispatcher([]*api.Rule{rule}, resolver).Run(file)
	return cols.Findings()
}

func TestUnsafeCallOnNullableType_IndexOperandOutsideMapsIsReported(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCallOnNullableType", `
package test

class Slot

class Holder(private val slots: Array<Slot?>, private val names: List<String?>, private val byKey: Map<String, Slot>) {
    fun arrayElement(): Slot = slots[0]!!
    fun listElement(): String = names[1]!!
    fun mapLookup(): Slot = byKey["a"]!!
}
`)
	got := bangBangRuleLines(findings, "UnsafeCallOnNullableType")
	if len(got) != 2 || got[0] != 7 || got[1] != 8 {
		t.Fatalf("want the array and list elements (lines 7, 8) and not the map lookup, got lines %v", got)
	}
}

func TestUnsafeCallOnNullableType_AssignmentTargetIsReported(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCallOnNullableType", `
package test

class Width(var value: Int)

class Holder {
    private var current: Width? = null

    fun assign(value: Int) {
        current!!.value = value
    }

    fun augmented(value: Int) {
        current!!.value += value
    }

    fun nonNull(other: Width) {
        other.value = 1
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("want the two assignment targets that start with current!!, got %d: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Col != 16 {
			t.Fatalf("want the finding on the !! token (column 16), got %+v", f)
		}
	}
}

// The compiler oracle renders a generic type with its arguments as the FQN, so the map check must read the base type.
func TestMapGetWithNotNullAssertion_OracleRenderedGenericReceiver(t *testing.T) {
	file := parseInline(t, `package test

fun lookup(byKey: Map<String, Int>): Int = byKey["a"]!!
`)
	var receiver uint32
	file.FlatWalkNodes(0, "indexing_expression", func(idx uint32) {
		if receiver == 0 {
			receiver = file.FlatFirstChild(idx)
		}
	})
	if receiver == 0 {
		t.Fatal("indexing expression not found")
	}
	o, err := oracle.LoadFromData(&oracle.Data{
		Version:       1,
		KotlinVersion: "2.1.0",
		Files: map[string]*oracle.File{
			file.Path: {Expressions: map[string]*oracle.ExpressionType{
				"3:44": {
					Type:      "kotlin.collections.Map<kotlin.String, kotlin.Int>",
					StartByte: int(file.FlatStartByte(receiver)),
					EndByte:   int(file.FlatEndByte(receiver)),
				},
			}},
		},
		Dependencies: map[string]*oracle.Class{},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	composite := oracle.NewCompositeResolver(o, resolver)
	for _, id := range []string{"MapGetWithNotNullAssertionOperator", "UnsafeCallOnNullableType"} {
		var rule *api.Rule
		for _, r := range api.Registry {
			if r.ID == id {
				rule = r
			}
		}
		if rule == nil {
			t.Fatalf("%s not registered", id)
		}
		got := bangBangRuleLines(runDispatcherFindings(rule, composite, file), id)
		want := 0
		if id == "MapGetWithNotNullAssertionOperator" {
			want = 1
		}
		if len(got) != want {
			t.Fatalf("%s: want %d findings on the map lookup, got lines %v", id, want, got)
		}
	}
}

func TestHasPlatformType_JavaConstructorAndCastAreNotPlatformTypes(t *testing.T) {
	findings := runRuleByName(t, "HasPlatformType", `
package test

class Holder {
    fun constructed() = java.util.ArrayList<String>()
    fun nestedConstructed() = java.util.AbstractMap.SimpleEntry("a", "b")
    fun cast() = java.util.Collections.emptyList<String>() as List<String>
    fun platform() = java.util.Collections.emptyList<String>()
    fun member() = java.io.File("a").name
}
`)
	got := bangBangRuleLines(findings, "HasPlatformType")
	if len(got) != 2 || got[0] != 8 || got[1] != 9 {
		t.Fatalf("want only the Java call results on lines 8 and 9, got lines %v", got)
	}
}

func TestCastNullableToNonNullableType_FunctionTypeTarget(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test

fun invoke(callback: Any?) {
    val run = callback as (() -> Unit)
    val maybe = callback as (() -> Unit)?
    run()
    maybe?.invoke()
}
`)
	got := bangBangRuleLines(findings, "CastNullableToNonNullableType")
	if len(got) != 1 || got[0] != 5 {
		t.Fatalf("want the cast to the non-null function type on line 5, got lines %v", got)
	}
}
