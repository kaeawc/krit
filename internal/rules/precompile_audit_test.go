package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestPrecompileDuplicateDeclaration_ExtensionReceiversAndExpectActualAreDistinct(t *testing.T) {
	findings := runRuleByName(t, "DuplicateDeclaration", `
package test
fun String.foo() = 1
fun Int.foo() = 2
val String.bar get() = 1
val Int.bar get() = 2
fun <T> List<T>.baz() = 1
fun <T> Set<T>.baz() = 1
expect fun plat(): Int
actual fun plat(): Int = 1
fun dup(x: Int) = 1
fun dup(x: Int) = 2
fun String.same() = 1
fun String.same() = 2
`)
	if len(findings) != 2 {
		t.Errorf("expected only the dup and same redeclarations, got %v", findings)
	}
}

func TestPrecompileUnreachableCode_LocalDeclarationsAfterJumpAreNotReported(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test
fun a(): Int {
    return 1
    fun local() {}
}
fun b() {
    return
    println()
}
`)
	if len(findings) != 1 {
		t.Errorf("expected only the println after return, got %v", findings)
	}
}

func TestPrecompileRulesAreSuppressedByKotlincDiagnosticNames(t *testing.T) {
	aliases := rules.AllSuppressionAliases()
	want := map[string]string{
		"UnreachableCode":           "UNREACHABLE_CODE",
		"DuplicateLabel":            "DUPLICATE_LABEL_IN_WHEN",
		"DuplicateDeclaration":      "CONFLICTING_OVERLOADS",
		"DeprecatedSymbolUsedError": "DEPRECATION_ERROR",
	}
	for rule, alias := range want {
		found := false
		for _, a := range aliases[rule] {
			if a == alias {
				found = true
			}
		}
		if !found {
			t.Errorf("%s should accept @Suppress(%q), aliases = %v", rule, alias, aliases[rule])
		}
	}
}

func TestPrecompileDeprecatedSymbolUsedError_OverloadsAndOtherOwnersAreNotReported(t *testing.T) {
	findings := runRuleByName(t, "DeprecatedSymbolUsedError", `
package test
@Deprecated("x", level = DeprecationLevel.ERROR)
fun old(a: Int) {}
fun old(a: String) {}

class K {
    @Deprecated("gone", level = DeprecationLevel.ERROR)
    fun helper() {}
}
class L {
    fun helper() {}
}

@Deprecated("y", level = DeprecationLevel.ERROR) fun solo() {}

fun use(k: K, l: L) {
    old("s")
    l.helper()
    k.helper()
    solo()
}
`)
	if len(findings) != 2 {
		t.Errorf("expected only k.helper() and solo(), got %v", findings)
	}
}

func TestPrecompileUnreachableCode_ProjectsTheCompilerVerdict(t *testing.T) {
	code := "package test\nfun stop(): Nothing = throw IllegalStateException()\nfun a() {\n    stop()\n    println()\n}\nfun b(): Int {\n    return 1\n    println()\n}\n"
	file := parseInline(t, code)
	fake := oracle.NewFakeOracle()
	// The compiler reports the statement after the Nothing-returning call, and nothing for `b`.
	fake.Diagnostics[file.Path] = []oracle.Diagnostic{{FactoryName: "UNREACHABLE_CODE", Severity: "WARNING", Line: 5, Col: 5}}
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{"4:5": {Name: "Nothing", FQN: "kotlin.Nothing", Kind: typeinfer.TypeNothing, Resolved: true}}
	findings := runRuleOnFileWithFakeOracle(t, "UnreachableCode", file, fake)
	if len(findings) != 1 || findings[0].Line != 5 || findings[0].Col != 5 {
		t.Fatalf("want one finding at 5:5 from the compiler, got %+v", findings)
	}
	// Without compiler facts the syntactic scan still reports the statement after `return`.
	findings = runRuleByName(t, "UnreachableCode", code)
	if len(findings) != 1 || findings[0].Line != 9 {
		t.Fatalf("want the syntactic finding at line 9, got %+v", findings)
	}
}

func unreachableVerdicts(t *testing.T, code string, line, col int) int {
	t.Helper()
	file := parseInline(t, code)
	fake := oracle.NewFakeOracle()
	fake.Diagnostics[file.Path] = []oracle.Diagnostic{{FactoryName: "UNREACHABLE_CODE", Severity: "WARNING", Line: line, Col: col}}
	return len(runRuleOnFileWithFakeOracle(t, "UnreachableCode", file, fake))
}

// A dependency that was not built gives the compiler an error type, which it treats as Nothing. The code after
// an elvis return or a try whose catch returns is not dead for that reason alone.
func TestPrecompileUnreachableCode_ErrorTypeDeadEndsAreNotReported(t *testing.T) {
	elvis := "package test\nfun a(items: List<String>): List<String> {\n    val posters = items\n        .filter { it.isNotEmpty() }\n    val first = posters.firstOrNull() ?: return emptyList()\n    val last = posters.lastOrNull()\n    return listOf(first)\n}\n"
	if n := unreachableVerdicts(t, elvis, 6, 5); n != 0 {
		t.Errorf("a statement after `x ?: return` is reachable, got %d findings", n)
	}
	tryCatch := "package test\nfun a(): Int {\n    val stored = try {\n        load()\n    } catch (e: IllegalStateException) {\n        return 1\n    }\n    return stored\n}\n"
	if n := unreachableVerdicts(t, tryCatch, 8, 5); n != 0 {
		t.Errorf("a try that only returns in its catch completes normally, got %d findings", n)
	}
}

func TestPrecompileUnreachableCode_KeepsJumpShapesTheCompilerReports(t *testing.T) {
	returnTry := "package test\nfun a(): Int {\n    return try {\n        return 1\n    } catch (e: Exception) {\n        throw e\n    }\n}\n"
	if n := unreachableVerdicts(t, returnTry, 3, 5); n != 1 {
		t.Errorf("`return try { return } catch { throw }` is reported by the compiler, got %d", n)
	}
	branches := "package test\nfun a(x: Int): Int {\n    if (x > 0) {\n        return 1\n    } else {\n        throw IllegalStateException()\n    }\n    println()\n    return 2\n}\n"
	if n := unreachableVerdicts(t, branches, 8, 5); n != 1 {
		t.Errorf("code after an if whose branches all jump is dead, got %d", n)
	}
	loop := "package test\nfun a() {\n    while (true) {\n        println()\n    }\n    println()\n}\n"
	if n := unreachableVerdicts(t, loop, 6, 5); n != 1 {
		t.Errorf("code after while (true) is dead, got %d", n)
	}
}
