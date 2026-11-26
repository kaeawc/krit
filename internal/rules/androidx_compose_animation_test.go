package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
)

// Lint's scope for ArcAnimationSpecTypeIssue and DisallowLookaheadAnimationVisualDebug is JAVA_FILE without
// TEST_SOURCES, so test sources are not checked.
func TestAnimationMainOnlyRulesSkipTests(t *testing.T) {
	arc := `package test

import androidx.compose.animation.core.ArcAnimationSpec

val spec = ArcAnimationSpec<Float>()
`
	if got := runAxlRule(t, "ArcAnimationSpecTypeIssue", "Spec.kt", arc, librarymodel.ProjectProfile{}); len(got) != 1 {
		t.Fatalf("main source: want 1 finding, got %v", got)
	}
	if got := runAxlRule(t, "ArcAnimationSpecTypeIssue", "src/test/kotlin/SpecTest.kt", arc, librarymodel.ProjectProfile{}); len(got) != 0 {
		t.Fatalf("test source: want no findings, got %v", got)
	}
	debug := `package test

import androidx.compose.animation.LookaheadAnimationVisualDebugging

fun f() { LookaheadAnimationVisualDebugging {} }
`
	if got := runAxlRule(t, "DisallowLookaheadAnimationVisualDebug", "Debug.kt", debug,
		librarymodel.ProjectProfile{}); len(got) != 1 {
		t.Fatalf("main source: want 1 finding, got %v", got)
	}
	if got := runAxlRule(t, "DisallowLookaheadAnimationVisualDebug", "src/test/kotlin/DebugTest.kt", debug,
		librarymodel.ProjectProfile{}); len(got) != 0 {
		t.Fatalf("test source: want no findings, got %v", got)
	}
}

// A star import does not make a same-file declaration of the type the animation class.
func TestArcAnimationSpecSameFileClassWins(t *testing.T) {
	src := `package test

import androidx.compose.animation.core.*

class ArcAnimationSpec<T>

val mine = ArcAnimationSpec<Float>()
`
	if got := runAxlRule(t, "ArcAnimationSpecTypeIssue", "Mine.kt", src, librarymodel.ProjectProfile{}); len(got) != 0 {
		t.Fatalf("same-file ArcAnimationSpec: want no findings, got %v", got)
	}
}
