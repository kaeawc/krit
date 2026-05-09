package rules_test

import (
	"testing"
)

func TestOverrideSignatureMismatch_TooManyParams_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "OverrideSignatureMismatch", `
package test

interface Greeter {
    fun greet(name: String): String
}

class WrongGreeter : Greeter {
    override fun greet(name: String, locale: String): String = "$name $locale"
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when override adds an extra parameter")
	}
}

func TestOverrideSignatureMismatch_TooFewParams_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "OverrideSignatureMismatch", `
package test

abstract class Base {
    abstract fun handle(x: Int, y: Int)
}

class Impl : Base() {
    override fun handle(x: Int) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when override drops a parameter")
	}
}

func TestOverrideSignatureMismatch_MatchingParams_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "OverrideSignatureMismatch", `
package test

interface Greeter {
    fun greet(name: String): String
}

class GoodGreeter : Greeter {
    override fun greet(name: String): String = "Hi $name"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings on matching override, got %d", len(findings))
	}
}

func TestOverrideSignatureMismatch_OverloadResolves_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "OverrideSignatureMismatch", `
package test

abstract class Multi {
    abstract fun handle()
    abstract fun handle(x: Int)
}

class HandleImpl : Multi() {
    override fun handle() {}
    override fun handle(x: Int) {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when each override matches one of the overloads, got %d", len(findings))
	}
}

func TestOverrideSignatureMismatch_LibrarySupertype_Negative(t *testing.T) {
	// Logger here has no source-side toString; the rule must stay silent
	// because no supertype member with the same name is visible.
	findings := runRuleByNameWithResolver(t, "OverrideSignatureMismatch", `
package test

abstract class Logger
class MyLogger : Logger() {
    override fun toString(): String = "MyLogger"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when supertype has no matching member name, got %d", len(findings))
	}
}

func TestOverrideSignatureMismatch_NotAnOverride_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "OverrideSignatureMismatch", `
package test

class Solo {
    fun greet(name: String): String = "Hi $name"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings on non-override function, got %d", len(findings))
	}
}
