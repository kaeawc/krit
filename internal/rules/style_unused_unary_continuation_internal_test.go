package rules

import (
	"testing"
)

// unusedVariableUsageForName parses code, locates the local variable
// declaration that binds `name`, and reports the usage verdict the
// UnusedVariable rule would compute for it.
func unusedVariableUsageForName(t *testing.T, code, name string) (used, unknown, found bool) {
	t.Helper()
	file := parseInlineForInternalTest(t, code)
	var result struct{ used, unknown, found bool }
	for _, nodeType := range []string{"property_declaration", "variable_declaration"} {
		file.FlatWalkNodes(0, nodeType, func(idx uint32) {
			if result.found {
				return
			}
			target, ok := unusedVariableDeclaration(file, idx)
			if !ok || target.name != name {
				return
			}
			used, unknown := unusedVariableUsage(file, target)
			result.used, result.unknown, result.found = used, unknown, true
		})
		if result.found {
			break
		}
	}
	return result.used, result.unknown, result.found
}

// TestUnusedVariableUnaryContinuation_BareName is the baseline already
// supported before this change: `+factoryCall` on the line after the
// declaration references the variable as the whole unary operand.
func TestUnusedVariableUnaryContinuation_BareName(t *testing.T) {
	code := "package test\n" +
		"fun f(b: Builder) {\n" +
		"    val factoryCall = b.create()\n" +
		"    +factoryCall\n" +
		"}\n"
	used, _, found := unusedVariableUsageForName(t, code, "factoryCall")
	if !found {
		t.Fatal("expected to find declaration of factoryCall")
	}
	if !used {
		t.Fatal("variable used as a bare unary continuation operand must count as used")
	}
}

// TestUnusedVariableUnaryContinuation_CallArgument pins the regression: the
// variable is nested as a call argument inside the unary operand
// (`+b.add(typeKey)`), not the whole operand. Before the fix the walker only
// matched a bare-identifier operand and reported a false positive.
func TestUnusedVariableUnaryContinuation_CallArgument(t *testing.T) {
	code := "package test\n" +
		"fun f(b: Builder) {\n" +
		"    val typeKey = b.key()\n" +
		"    +b.add(typeKey)\n" +
		"}\n"
	used, _, found := unusedVariableUsageForName(t, code, "typeKey")
	if !found {
		t.Fatal("expected to find declaration of typeKey")
	}
	if !used {
		t.Fatal("variable used as a call argument inside the unary operand must count as used")
	}
}

// TestUnusedVariableUnaryContinuation_LambdaBody covers a reference inside a
// trailing lambda of the unary operand (`+b.apply { instance.register() }`).
func TestUnusedVariableUnaryContinuation_LambdaBody(t *testing.T) {
	code := "package test\n" +
		"fun f(b: Builder) {\n" +
		"    val instance = b.create()\n" +
		"    +b.apply { instance.register() }\n" +
		"}\n"
	used, _, found := unusedVariableUsageForName(t, code, "instance")
	if !found {
		t.Fatal("expected to find declaration of instance")
	}
	if !used {
		t.Fatal("variable referenced inside the unary operand's trailing lambda must count as used")
	}
}

// TestUnusedVariableUnaryContinuation_GenuinelyUnused guards against
// over-suppression: a unary continuation that does NOT reference the variable
// must leave it reported as unused.
func TestUnusedVariableUnaryContinuation_GenuinelyUnused(t *testing.T) {
	code := "package test\n" +
		"fun f(b: Builder) {\n" +
		"    val dead = b.create()\n" +
		"    +b.add(0)\n" +
		"}\n"
	used, unknown, found := unusedVariableUsageForName(t, code, "dead")
	if !found {
		t.Fatal("expected to find declaration of dead")
	}
	if used || unknown {
		t.Fatalf("a variable absent from the unary continuation must remain unused (used=%v unknown=%v)", used, unknown)
	}
}
