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
