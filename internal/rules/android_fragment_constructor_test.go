package rules_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// The reported lines are those Android Lint 32.4.1 reports for the same classes, minus the divergences the
// rule documents: Kotlin internal members compile to public JVM members and
// all-default primary constructors have a JVM no-arg overload.
func TestFragmentConstructor_VisibilityMatchesLint(t *testing.T) {
	findings := runRuleByName(t, "FragmentConstructor", `package p

import android.app.Fragment

class Public1 : Fragment()
internal class Internal1 : Fragment()
private class Private1 : Fragment()
abstract class Abstract1 : Fragment()
class ChildPrivCtor private constructor() : Fragment()
class ChildInternalCtor internal constructor() : Fragment()
class ChildProtCtor : Fragment {
    protected constructor() : super()
}
class Outer {
    class Nested : Fragment()
    inner class Inner1 : Fragment()
    private class PrivNested : Fragment()
    protected class ProtNested : Fragment()
    internal class IntNested : Fragment()
    fun f() {
        class Local : Fragment()
        val anon = object : Fragment() {}
    }
}
class PrivWithArg private constructor(val s: String) : Fragment()
class PrivAndPublic private constructor(val s: String) : Fragment() {
    constructor() : this("")
}
class AllDefaults(val x: Int = 1) : Fragment()
`)
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	slices.Sort(lines)
	if want := []int{7, 9, 12, 16, 17, 18, 22, 25}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines = %v, want %v", lines, want)
	}
	if !strings.Contains(findingMessageAtLine(findings, 7), "should be public (Private1)") {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func findingMessageAtLine(findings []scanner.Finding, line int) string {
	for _, f := range findings {
		if f.Line == line {
			return f.Message
		}
	}
	return ""
}

func TestFragmentConstructor_InjectedFragmentSkipsVisibilityChecks(t *testing.T) {
	findings := runRuleByName(t, "FragmentConstructor", `package p

import android.app.Fragment
import javax.inject.Inject

class Outer {
    private class Injected @Inject constructor(val s: String) : Fragment()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected a custom FragmentFactory fragment to be skipped, got %v", findings)
	}
}
