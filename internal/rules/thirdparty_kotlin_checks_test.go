package rules_test

import "testing"

// ExceptionMessageDetector does not run on test sources.
func TestExceptionMessageSkipsTestSources(t *testing.T) {
	const src = "package app\n\nfun f(x: Int?) {\n  requireNotNull(x)\n}\n"
	if got := runRuleByNameOnPath(t, "ExceptionMessage", "src/main/java/app/F.kt", src); len(got) != 1 {
		t.Fatalf("main source: want one finding, got %v", got)
	}
	if got := runRuleByNameOnPath(t, "ExceptionMessage", "src/test/java/app/FTest.kt", src); len(got) != 0 {
		t.Errorf("test source: want no finding, got %v", got)
	}
}

// A precondition imported from elsewhere is not kotlin's.
func TestExceptionMessageRequiresTheStdlibFunction(t *testing.T) {
	const src = "package app\n\nimport org.junit.Assume.check\n\nfun f(x: Boolean) {\n  check(x)\n}\n"
	if got := runRuleByNameOnPath(t, "ExceptionMessage", "src/main/java/app/F.kt", src); len(got) != 0 {
		t.Errorf("want no finding for an imported check(), got %v", got)
	}
}

// Only a function in scope at the call shadows the stdlib precondition: an unrelated class member does not.
func TestExceptionMessageShadowingScope(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      int
	}{
		{"unrelated member", "class Other { fun check(x: Boolean) = x }\n\nfun f(x: Boolean) {\n  check(x)\n}\n", 1},
		{"member of the enclosing class", "class Owner {\n  fun check(x: Boolean) = x\n  fun f(x: Boolean) {\n    check(x)\n  }\n}\n", 0},
		{"top-level function of the file", "fun check(x: Boolean) = x\n\nfun f(x: Boolean) {\n  check(x)\n}\n", 0},
		{"explicit kotlin import beats a top-level function", "import kotlin.check\n\nfun <T> check(x: T) = x\n\nfun f(x: Boolean) {\n  check(x)\n}\n", 1},
		{"local function", "fun f(x: Boolean) {\n  fun check(y: Boolean) = y\n  check(x)\n}\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := runRuleByNameOnPath(t, "ExceptionMessage", "src/main/java/app/F.kt", "package app\n\n"+tc.src); len(got) != tc.want {
				t.Errorf("want %d findings, got %v", tc.want, got)
			}
		})
	}
}

// NullableConcurrentHashMap is Java-capable: a local class named ConcurrentHashMap is not java.util.concurrent's.
func TestNullableConcurrentHashMapJavaLookalike(t *testing.T) {
	const java = `package app;

class ConcurrentHashMap<K, V> {}

class J {
  ConcurrentHashMap<@Nullable String, Integer> field;
}
`
	if got := runRuleByNameOnJava(t, "NullableConcurrentHashMap", java); len(got) != 0 {
		t.Errorf("want no finding for a local ConcurrentHashMap, got %v", got)
	}
}

// Review of #1799 (P3): a function-typed parameter or local variable named like the precondition shadows it.
func TestExceptionMessageShadowedByVariables(t *testing.T) {
	for name, src := range map[string]string{
		"parameter":        "fun f(require: (Boolean) -> Unit) {\n  require(true)\n}\n",
		"local":            "fun f() {\n  val check: (Boolean) -> Unit = {}\n  check(true)\n}\n",
		"lambda parameter": "val g: ((Boolean) -> Unit) -> Unit = { check -> check(true) }\n",
	} {
		if got := runRuleByNameOnPath(t, "ExceptionMessage", "src/main/java/app/F.kt", "package app\n\n"+src); len(got) != 0 {
			t.Errorf("%s: want no finding, got %v", name, got)
		}
	}
}

// Review of #1799 (P1): a type annotation such as @RawValue is not a function type; P3: a fully qualified
// ConcurrentHashMap type is still the class.
func TestKotlinChecksTypeShapes(t *testing.T) {
	const parcel = `package app

import android.os.Parcelable
import kotlinx.parcelize.Parcelize
import kotlinx.parcelize.RawValue

class Foo

@Parcelize
class P(
  val a: @RawValue Foo,
  val b: @RawValue List<String>,
  val c: @RawValue (() -> Unit)?,
  val d: suspend () -> Unit,
) : Parcelable
`
	got := runRuleByName(t, "ParcelizeFunctionProperty", parcel)
	if len(got) != 2 || got[0].Line != 13 || got[1].Line != 14 {
		t.Errorf("ParcelizeFunctionProperty: want lines 13 and 14, got %v", got)
	}
	const maps = `package app

class C(val n: java.util.concurrent.ConcurrentHashMap<String?, Int>) {
  val m: java.util.concurrent.ConcurrentHashMap<String, Int?> = java.util.concurrent.ConcurrentHashMap()
}
`
	if got := runRuleByName(t, "NullableConcurrentHashMap", maps); len(got) != 2 {
		t.Errorf("NullableConcurrentHashMap: want 2 findings for fully qualified types, got %v", got)
	}
}
