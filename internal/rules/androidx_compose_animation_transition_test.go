package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
)

// A with/apply/run lambda between the call and the Transition extension brings a closer implicit receiver, whose
// own animateFloat member the call may resolve to (first-pass review of #1734).
func TestUnusedTransitionTargetStateReceiverLambdaBetween(t *testing.T) {
	src := `package test

import androidx.compose.animation.core.Transition
import androidx.compose.animation.core.animateFloat

class Animator {
    fun animateFloat(block: (Boolean) -> Float): Float = block(true)
}

fun Transition<Boolean>.direct() {
    animateFloat(label = "a") { 1f }
}

fun Transition<Boolean>.through(other: Animator) {
    with(other) { animateFloat { 1f } }
    other.apply { this.animateFloat { 1f } }
    other.run { animateFloat { 1f } }
}
`
	got := runAxlRule(t, "UnusedTransitionTargetStateParameter", "T.kt", src, librarymodel.ProjectProfile{})
	if len(got) != 1 || got[0].Line != 11 {
		t.Fatalf("want only the direct call (line 11), got %v", got)
	}
}
