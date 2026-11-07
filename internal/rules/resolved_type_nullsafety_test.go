package rules_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestNullSafetyAbsenceClaimsRequireResolvedTypes(t *testing.T) {
	tests := []struct {
		name       string
		rule       string
		unresolved string
		resolved   string
	}{
		{
			name: "unnecessary null check",
			rule: "UnnecessaryNotNullCheck",
			unresolved: `package test
fun check(value: ExternalThing) {
    if (value != null) println(value)
}`,
			resolved: `package test
class KnownThing
fun check(value: KnownThing) {
    if (value != null) println(value)
}`,
		},
		{
			name: "unnecessary not-null operator",
			rule: "UnnecessaryNotNullOperator",
			unresolved: `package test
fun check(value: ExternalThing) {
    println(value!!)
}`,
			resolved: `package test
class KnownThing
fun check(value: KnownThing) {
    println(value!!)
}`,
		},
		{
			name: "unnecessary safe call",
			rule: "UnnecessarySafeCall",
			unresolved: `package test
fun check(value: ExternalThing) {
    println(value?.toString())
}`,
			resolved: `package test
class KnownThing
fun check(value: KnownThing) {
    println(value?.toString())
}`,
		},
		{
			name: "useless elvis",
			rule: "UselessElvisOnNonNull",
			unresolved: `package test
fun check(value: ExternalThing, fallback: ExternalThing): ExternalThing {
    return value ?: fallback
}`,
			resolved: `package test
class KnownThing
fun check(value: KnownThing, fallback: KnownThing): KnownThing {
    return value ?: fallback
}`,
		},
		{
			name: "nullable cast to non-null target",
			rule: "CastNullableToNonNullableType",
			unresolved: `package test
fun check(value: String?) {
    println(value as ExternalThing)
}`,
			resolved: `package test
class KnownThing
fun check(value: String?) {
    println(value as KnownThing)
}`,
		},
		{
			name: "of-not-null factory",
			rule: "UselessCallOnNotNull",
			unresolved: `package test
fun check(value: ExternalThing) {
    println(listOfNotNull(value))
}`,
			resolved: `package test
class KnownThing
fun check(value: KnownThing) {
    println(listOfNotNull(value))
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if findings := runRuleByNameWithResolver(t, tt.rule, tt.unresolved); len(findings) != 0 {
				t.Fatalf("unresolved type produced %d findings: %v", len(findings), findings)
			}
			if findings := runRuleByNameWithResolver(t, tt.rule, tt.resolved); len(findings) == 0 {
				t.Fatal("genuinely resolved non-null type produced no finding")
			}
		})
	}
}

func TestUselessCallOnNotNull_OfNotNullWithNullableMemberArguments(t *testing.T) {
	sources := map[string]string{
		"member property of another class": `package test
class Pos
sealed interface Marker
class RangeMarker(val start: Pos?, val end: Pos?) : Marker
class PointMarker(val at: Pos) : Marker
fun Marker.positions(): List<Pos> = when (this) {
    is RangeMarker -> listOfNotNull(start)
    is PointMarker -> listOf(at)
}`,
		"two member properties": `package test
class Tag
class FilterSpec(val include: Tag? = null, val exclude: Tag? = null) {
    fun tags(): List<Tag> = listOfNotNull(include, exclude)
}`,
		"tag parameters": `package test
class Tag
fun build(include: Tag?, exclude: Tag?): List<Tag> = listOfNotNull(include, exclude)`,
		"member of external type": `package test
fun build(config: ExternalConfig): List<String> = listOfNotNull(config.name, config.title)`,
		"unresolvable property": `package test
class Holder {
    fun all(): List<Thing> = listOfNotNull(first, second)
}`,
	}
	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			if findings := runRuleByNameWithResolver(t, "UselessCallOnNotNull", src); len(findings) != 0 {
				t.Fatalf("nullable or unresolved argument produced %d findings: %v", len(findings), findings)
			}
		})
	}
}

// An oracle type for a bare identifier is not trusted when the file does not declare that name: the property
// may belong to another class and be nullable.
func TestUselessCallOnNotNull_OfNotNullOracleTypeForUndeclaredIdentifier(t *testing.T) {
	run := func(code string) []scanner.Finding {
		file := parseInline(t, code)
		resolver := typeinfer.NewResolver()
		resolver.IndexFilesParallel([]*scanner.File{file}, 1)
		fake := oracle.NewFakeOracle()
		fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{}
		file.FlatWalkNodes(0, "simple_identifier", func(idx uint32) {
			if strings.TrimSpace(file.FlatNodeText(idx)) == "from" || strings.TrimSpace(file.FlatNodeText(idx)) == "value" {
				key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
				fake.Expressions[file.Path][key] = &typeinfer.ResolvedType{Name: "Point", Kind: typeinfer.TypeClass, Resolved: true}
			}
		})
		composite := oracle.NewCompositeResolver(fake, resolver)
		for _, r := range api.Registry {
			if r.ID == "UselessCallOnNotNull" {
				cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
				return cols.Findings()
			}
		}
		t.Fatal("rule not registered")
		return nil
	}
	if got := run(`package test
fun Command.points() = listOfNotNull(from)`); len(got) != 0 {
		t.Fatalf("undeclared identifier must stay silent, got %d", len(got))
	}
	if got := run(`package test
class Point
fun points(value: Point) = listOfNotNull(value)`); len(got) != 1 {
		t.Fatalf("declared non-null parameter should be reported, got %d", len(got))
	}
	if got := run(`package test
class Point
fun points(value: Point?) = listOfNotNull(value)`); len(got) != 0 {
		t.Fatalf("declared nullable parameter must stay silent, got %d", len(got))
	}
}
