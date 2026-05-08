package arch

import (
	"testing"
)

func TestDetectLeaky_NotLeaky(t *testing.T) {
	lines := []string{
		"class UserService(private val repo: UserRepo) {",
		"    fun getUser(id: String): User {",
		"        val cached = cache.get(id)",
		"        if (cached != null) return cached",
		"        return repo.findById(id)",
		"    }",
		"}",
	}

	results := DetectLeakyAbstractions(lines, 0.8)
	if len(results) != 0 {
		t.Errorf("expected no leaky classes, got %d", len(results))
	}
}

func TestDetectLeaky_FullDelegation(t *testing.T) {
	lines := []string{
		"class UserServiceWrapper(private val delegate: UserService) {",
		"    fun getUser(id: String): User = delegate.getUser(id)",
		"    fun saveUser(user: User) = delegate.saveUser(user)",
		"    fun deleteUser(id: String) = delegate.deleteUser(id)",
		"}",
	}

	results := DetectLeakyAbstractions(lines, 0.8)
	if len(results) != 1 {
		t.Fatalf("expected 1 leaky class, got %d", len(results))
	}

	r := results[0]
	if r.ClassName != "UserServiceWrapper" {
		t.Errorf("expected ClassName=UserServiceWrapper, got %s", r.ClassName)
	}
	if r.WrappedType != "UserService" {
		t.Errorf("expected WrappedType=UserService, got %s", r.WrappedType)
	}
	if r.TotalMethods != 3 {
		t.Errorf("expected TotalMethods=3, got %d", r.TotalMethods)
	}
	if r.DelegatingMethods != 3 {
		t.Errorf("expected DelegatingMethods=3, got %d", r.DelegatingMethods)
	}
	if r.DelegationRatio != 1.0 {
		t.Errorf("expected DelegationRatio=1.0, got %f", r.DelegationRatio)
	}
}

func TestDetectLeaky_PartialDelegation_BelowThreshold(t *testing.T) {
	lines := []string{
		"class Service(private val delegate: Inner) {",
		"    fun a() = delegate.a()",
		"    fun b(): String {",
		"        return computeSomething()",
		"    }",
		"    fun c(): Int {",
		"        return 42",
		"    }",
		"    fun d(): Boolean {",
		"        return true",
		"    }",
		"}",
	}

	results := DetectLeakyAbstractions(lines, 0.8)
	if len(results) != 0 {
		t.Errorf("expected no leaky classes (1/4 = 0.25 < 0.8), got %d", len(results))
	}
}

func TestDetectLeaky_NoPrivateField(t *testing.T) {
	// Public constructor param (no val) — not a stored field
	lines := []string{
		"class Wrapper(delegate: Inner) {",
		"    fun doStuff() = delegate.doStuff()",
		"}",
	}

	results := DetectLeakyAbstractions(lines, 0.5)
	if len(results) != 0 {
		t.Errorf("expected no leaky classes for public param, got %d", len(results))
	}
}

func TestDetectLeaky_AbstractClass(t *testing.T) {
	lines := []string{
		"abstract class BaseWrapper(private val delegate: Inner) {",
		"    fun doStuff() = delegate.doStuff()",
		"}",
	}

	results := DetectLeakyAbstractions(lines, 0.5)
	if len(results) != 0 {
		t.Errorf("expected no leaky classes for abstract class, got %d", len(results))
	}
}

func TestDetectLeaky_MultipleConstructorParams(t *testing.T) {
	lines := []string{
		"class Service(private val a: TypeA, private val b: TypeB) {",
		"    fun doA() = a.doA()",
		"    fun doB() = b.doB()",
		"}",
	}

	results := DetectLeakyAbstractions(lines, 0.5)
	if len(results) != 0 {
		t.Errorf("expected no leaky classes for multi-param constructor, got %d", len(results))
	}
}
