package rules

import (
	"context"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func runLeakyAbstractionRule(t *testing.T, src string) []scanner.Finding {
	t.Helper()
	parser := scanner.GetKotlinParser()
	defer scanner.PutKotlinParser(parser)
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	file := scanner.NewParsedFile("X.kt", []byte(src), tree)
	rule := &PublicToInternalLeakyAbstractionRule{
		BaseRule:  BaseRule{RuleName: "PublicToInternalLeakyAbstraction", RuleSetName: "architecture", Sev: "info"},
		Threshold: 0.80,
	}
	apiRule := &api.Rule{
		ID:        rule.RuleName,
		Category:  rule.RuleSetName,
		Sev:       api.Severity(rule.Sev),
		NodeTypes: []string{"class_declaration"},
		Check:     rule.check,
	}
	cols := NewDispatcher([]*api.Rule{apiRule}, nil).Run(file)
	return cols.Findings()
}

func TestLeakyAbstraction_FullDelegation(t *testing.T) {
	src := `class UserService(private val impl: InternalUserService) {
    fun get(id: Long): User = impl.get(id)
    fun save(user: User) { impl.save(user) }
    fun delete(id: Long): Int { return impl.delete(id) }
}`
	if got := runLeakyAbstractionRule(t, src); len(got) != 1 {
		t.Fatalf("want 1 finding, got %d (%v)", len(got), got)
	}
}

func TestLeakyAbstraction_BelowThreshold(t *testing.T) {
	src := `class Service(private val delegate: Inner) {
    fun a() = delegate.a()
    fun b(): String { return computeSomething() }
    fun c(): Int { return 42 }
    fun d(): Boolean { return true }
}`
	if got := runLeakyAbstractionRule(t, src); len(got) != 0 {
		t.Fatalf("want 0 findings (1/4 below 0.80), got %d", len(got))
	}
}

func TestLeakyAbstraction_AbstractSealedDataEnumValueAnnotation(t *testing.T) {
	cases := []string{
		`abstract class Wrapper(private val impl: Inner) { fun a() = impl.a() }`,
		`sealed class Wrapper(private val impl: Inner) { fun a() = impl.a() }`,
		`data class Wrapper(private val impl: Inner) { fun a() = impl.a() }`,
		`enum class Wrapper(private val impl: Inner) { A; fun a() = impl.a() }`,
		`value class Wrapper(private val impl: Inner) { fun a() = impl.a() }`,
		`annotation class Wrapper(private val impl: Inner)`,
	}
	for _, src := range cases {
		if got := runLeakyAbstractionRule(t, src); len(got) != 0 {
			t.Errorf("expected no findings for excluded class kind, got %d for %q", len(got), src)
		}
	}
}

func TestLeakyAbstraction_NonPublicClasses(t *testing.T) {
	for _, vis := range []string{"private", "internal", "protected"} {
		src := vis + ` class Wrapper(private val impl: Inner) {
    fun a() = impl.a()
    fun b() = impl.b()
}`
		if got := runLeakyAbstractionRule(t, src); len(got) != 0 {
			t.Errorf("expected no findings for %s class, got %d", vis, len(got))
		}
	}
}

func TestLeakyAbstraction_ConstructorScopedParameterNotStoredProperty(t *testing.T) {
	src := `class Wrapper(delegate: Inner) {
    private val impl = delegate
    fun doStuff() = impl.doStuff()
    fun more() = impl.more()
}`
	if got := runLeakyAbstractionRule(t, src); len(got) != 0 {
		t.Errorf("want no findings when ctor param isn't a stored val, got %d", len(got))
	}
}

func TestLeakyAbstraction_VarParameterSkipped(t *testing.T) {
	src := `class Wrapper(private var impl: Inner) {
    fun a() = impl.a()
    fun b() = impl.b()
}`
	if got := runLeakyAbstractionRule(t, src); len(got) != 0 {
		t.Errorf("want no findings for var-backed wrapper, got %d", len(got))
	}
}

func TestLeakyAbstraction_MultipleConstructorParams(t *testing.T) {
	src := `class Service(private val a: TypeA, private val b: TypeB) {
    fun doA() = a.doA()
    fun doB() = b.doB()
}`
	if got := runLeakyAbstractionRule(t, src); len(got) != 0 {
		t.Errorf("want no findings for multi-param ctor, got %d", len(got))
	}
}

func TestLeakyAbstraction_NestedClassMethodsExcluded(t *testing.T) {
	// Nested classes have their own surface; their methods must not be
	// counted toward the outer class. The outer class here has 0
	// methods declared directly, so nothing to flag.
	src := `class Outer(private val impl: Inner) {
    class Helper {
        fun a() = otherImpl.a()
        fun b() = otherImpl.b()
    }
}`
	if got := runLeakyAbstractionRule(t, src); len(got) != 0 {
		t.Errorf("want no findings for outer class with only nested-class methods, got %d", len(got))
	}
}

func TestLeakyAbstraction_NestedClassNotFlaggedIndependently(t *testing.T) {
	src := `class Outer(private val cfg: Config) {
    fun configName(): String = cfg.name()
    class Inner(private val impl: Service) {
        fun a() = impl.a()
        fun b() = impl.b()
    }
}`
	got := runLeakyAbstractionRule(t, src)
	// Outer has a single method which IS a delegation; Inner is nested so
	// must be ignored. Outer's 1/1 ratio > 0.80 → expect exactly one finding
	// for Outer (not for Inner).
	if len(got) != 1 {
		t.Fatalf("want 1 finding for Outer only, got %d (%v)", len(got), got)
	}
}

func TestLeakyAbstraction_ChainedCallIsNotDelegation(t *testing.T) {
	// `impl.get(id).normalize()` adds real behavior — the outer call's
	// receiver is itself a call_expression, so the navigation chain has
	// more than two segments.
	src := `class Wrapper(private val impl: Inner) {
    fun get(id: Long): String = impl.get(id).normalize()
    fun save(user: User) { impl.save(user) }
}`
	if got := runLeakyAbstractionRule(t, src); len(got) != 0 {
		t.Errorf("want no findings when only half are pure delegations, got %d", len(got))
	}
}

func TestLeakyAbstraction_MultiStatementBlockNotDelegation(t *testing.T) {
	src := `class Wrapper(private val impl: Inner) {
    fun a() = impl.a()
    fun b() {
        log("calling b")
        impl.b()
    }
}`
	// 1 delegating / 2 total = 0.5 < 0.80 → no finding.
	if got := runLeakyAbstractionRule(t, src); len(got) != 0 {
		t.Errorf("want no findings (multi-statement block not delegation), got %d", len(got))
	}
}

func TestLeakyAbstraction_PrivateMethodsNotCounted(t *testing.T) {
	src := `class Wrapper(private val impl: Inner) {
    fun a() = impl.a()
    fun b() = impl.b()
    private fun helper(): Int { return 42 }
}`
	if got := runLeakyAbstractionRule(t, src); len(got) != 1 {
		t.Fatalf("want 1 finding (private helper ignored), got %d", len(got))
	}
}

func TestLeakyAbstraction_NonValParameterWithDelegation(t *testing.T) {
	// Constructor param with no `val`/`var` is not a stored property,
	// so even if methods reference it lexically the rule shouldn't fire.
	src := `class Wrapper(impl: Inner) {
    private val stored = impl
    fun a() = stored.a()
    fun b() = stored.b()
}`
	if got := runLeakyAbstractionRule(t, src); len(got) != 0 {
		t.Errorf("want no findings: ctor param has no val/var, got %d", len(got))
	}
}

func TestLeakyAbstraction_InnerClassSkipped(t *testing.T) {
	src := `class Outer {
    inner class Wrapper(private val impl: Inner) {
        fun a() = impl.a()
        fun b() = impl.b()
    }
}`
	if got := runLeakyAbstractionRule(t, src); len(got) != 0 {
		t.Errorf("want no findings for inner class wrapper, got %d", len(got))
	}
}

func TestLeakyAbstraction_NoMethodsNoFinding(t *testing.T) {
	src := `class Holder(private val impl: Inner)`
	if got := runLeakyAbstractionRule(t, src); len(got) != 0 {
		t.Errorf("want no findings for empty wrapper, got %d", len(got))
	}
}
