package rules_test

import (
	"testing"
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

// --- UnnecessaryApply ---

func TestUnnecessaryApply_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryApply", `
package test
fun example() {
    val x = StringBuilder().apply {}
}`)
	if len(findings) == 0 {
		t.Error("expected UnnecessaryApply to flag empty '.apply {}'")
	}
}

func TestUnnecessaryApply_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryApply", `
package test
fun example() {
    val x = StringBuilder().apply {
        this.append("hello")
    }
}`)
	if len(findings) > 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

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
        upper + "!"
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
