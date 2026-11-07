package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// --- RedundantHigherOrderMapUsage ---

func TestRedundantHigherOrderMapUsage_Positive(t *testing.T) {
	findings := runRuleByName(t, "RedundantHigherOrderMapUsage", `
package test
fun example() {
    val result = listOf(1, 2, 3).map { it }
}`)
	if len(findings) == 0 {
		t.Error("expected RedundantHigherOrderMapUsage to flag '.map { it }'")
	}
}

func TestRedundantHigherOrderMapUsage_Negative(t *testing.T) {
	findings := runRuleByName(t, "RedundantHigherOrderMapUsage", `
package test
fun example() {
    val result = listOf(1, 2, 3).map { it * 2 }
}`)
	if len(findings) > 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestRedundantHigherOrderMapUsage_NegativeNonListReceivers(t *testing.T) {
	findings := runRuleByName(t, "RedundantHigherOrderMapUsage", `
package test
fun example(names: Set<String>, lookup: Map<String, Int>, items: List<Int>) {
    val a = names.map { it }
    val b = lookup.values.map { it }
    val c = items.map<Int, Number> { it }
    val d = unknownSource().map { it }
}`)
	if len(findings) > 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestRedundantHigherOrderMapUsage_PositiveDeclaredListParameter(t *testing.T) {
	findings := runRuleByName(t, "RedundantHigherOrderMapUsage", `
package test
fun example(items: List<Int>) {
    val a = items.map { it }
}`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

// --- UnnecessaryApply ---

// --- UnnecessaryLet ---

func TestUnnecessaryLet_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryLet", `
package test
fun example() {
    val x = "hello".let { it }
}`)
	if len(findings) == 0 {
		t.Error("expected UnnecessaryLet to flag '.let { it }'")
	}
}

func TestUnnecessaryLet_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryLet", `
package test
fun example() {
    val x = "hello".let {
        val upper = it.uppercase()
        upper + it.length
    }
}`)
	if len(findings) > 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// --- UnnecessaryFilter ---

func TestUnnecessaryFilter_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryFilter", `
package test
fun example() {
    val x = listOf(1, 2, 3).filter { it > 1 }.first()
}`)
	if len(findings) == 0 {
		t.Error("expected UnnecessaryFilter to flag '.filter { ... }.first()'")
	}
}

func TestUnnecessaryFilter_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryFilter", `
package test
fun example() {
    val x = listOf(1, 2, 3).first { it > 1 }
}`)
	if len(findings) > 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// --- UnnecessaryFullyQualifiedName ---

func TestUnnecessaryFullyQualifiedName_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryFullyQualifiedName", `
package test
import kotlin.collections.listOf
fun example() {
    val x = kotlin.collections.listOf(1, 2, 3)
}`)
	if len(findings) == 0 {
		t.Error("expected UnnecessaryFullyQualifiedName to flag FQN when import exists")
	}
}

func TestUnnecessaryFullyQualifiedName_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryFullyQualifiedName", `
package test
import kotlin.collections.listOf
fun example() {
    val x = listOf(1, 2, 3)
}`)
	if len(findings) > 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// --- UnnecessaryAny ---

func TestUnnecessaryAny_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryAny", `
package test
fun example() {
    val x = listOf(1, 2, 3).any { true }
}`)
	if len(findings) == 0 {
		t.Error("expected UnnecessaryAny to flag '.any { true }'")
	}
}

func TestUnnecessaryAny_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryAny", `
package test
fun example() {
    val x = listOf(1, 2, 3).any { it > 2 }
}`)
	if len(findings) > 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// --- UnnecessaryBracesAroundTrailingLambda ---

func TestUnnecessaryBracesAroundTrailingLambda_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryBracesAroundTrailingLambda", `
package test
fun example() {
    listOf(1, 2, 3).forEach() { println(it) }
}`)
	if len(findings) == 0 {
		t.Error("expected UnnecessaryBracesAroundTrailingLambda to flag 'forEach() { ... }'")
	}
}

func TestUnnecessaryBracesAroundTrailingLambda_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryBracesAroundTrailingLambda", `
package test
fun example() {
    listOf(1, 2, 3).forEach { println(it) }
}`)
	if len(findings) > 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// --- UnnecessaryReversed ---

func TestUnnecessaryReversed_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryReversed", `
package test
fun example() {
    val x = listOf(3, 1, 2).sorted().reversed()
}`)
	if len(findings) == 0 {
		t.Error("expected UnnecessaryReversed to flag '.sorted().reversed()'")
	}
}

func TestUnnecessaryReversed_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryReversed", `
package test
fun example() {
    val x = listOf(3, 1, 2).sortedDescending()
}`)
	if len(findings) > 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestUnnecessaryAny_AnyItOnBooleansIsNotEmptiness(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryAny", `
package test
fun f(flags: List<Boolean>) = flags.any { it }
`)
	if len(findings) != 0 {
		t.Fatalf("`any { it }` is not `isNotEmpty()`, got %d findings", len(findings))
	}
}

func TestUnnecessaryBracesAroundTrailingLambda_SkipsDelegateBeforeClassBody(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryBracesAroundTrailingLambda", `
package test
class Fake : Helper by ditto() {
    fun f(l: List<Int>) = l.none() { it > 1 }
}
`)
	if len(findings) != 1 || findings[0].Line != 4 {
		t.Fatalf("expected only the real trailing lambda call on line 4, got %+v", findings)
	}
}

func TestUnnecessaryFullyQualifiedName_MatchesWholeImportedName(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryFullyQualifiedName", `
package test
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import java.util.Date as UtilDate

fun example() {
    androidx.compose.material3.IconButton(1)
    androidx.compose.material3.IconButton.Companion.make()
    java.util.Date()
}`)
	if len(findings) != 2 {
		t.Fatalf("expected one finding per IconButton use and none for the aliased Date, got %d: %v", len(findings), findings)
	}
	for _, f := range findings {
		if !strings.Contains(f.Message, "'androidx.compose.material3.IconButton'") || !strings.Contains(f.Message, "Use 'IconButton'") {
			t.Errorf("unexpected message %q", f.Message)
		}
	}
}

func TestUnnecessaryFullyQualifiedName_ReportsOncePerUseAndRespectsBoundaries(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryFullyQualifiedName", `
package test

import java.util.Date
import java.util.concurrent.TimeUnit
import foo.Local
import foo.bar.Aliased as Other

class Local

val a = java.util.Date()
val b = java.util.concurrent.TimeUnit.SECONDS
val c = java.util.DateFormat.getInstance()
val d = foo.Local()
val e: java.util.Date? = null
val f = "java.util.Date"
val g = foo.bar.Aliased()
`)
	if len(findings) != 3 {
		t.Fatalf("expected Date (value), TimeUnit and Date (type) once each, got %d: %v", len(findings), findings)
	}
}

func TestUnnecessaryAny_BooleanElementsAndFix(t *testing.T) {
	src := `package test
fun f(flags: List<Boolean>, xs: Iterable<Int>) {
    flags.any { it }
    xs.any { true }
    xs.none { true }
}
`
	findings := runRuleByName(t, "UnnecessaryAny", src)
	if len(findings) != 2 {
		t.Fatalf("expected findings for the two { true } calls only, got %v", findings)
	}
	apply := func(f scanner.Finding) string { return src[:f.Fix.StartByte] + f.Fix.Replacement + src[f.Fix.EndByte:] }
	if got := apply(findings[0]); !strings.Contains(got, "xs.any()") {
		t.Errorf("unexpected fix: %s", got)
	}
	if got := apply(findings[1]); !strings.Contains(got, "xs.none()") {
		t.Errorf("unexpected fix: %s", got)
	}
}
