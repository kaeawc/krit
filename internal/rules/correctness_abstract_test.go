package rules_test

import (
	"strings"
	"testing"
)

func TestAbstractMemberNotImplemented_MissingInterfaceMethod_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "AbstractMemberNotImplemented", `
package test

interface Greeter {
    fun greet(name: String): String
    fun farewell()
}

class HollowGreeter : Greeter
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when class fails to implement interface members")
	}
	if !strings.Contains(findings[0].Message, "greet") || !strings.Contains(findings[0].Message, "farewell") {
		t.Errorf("expected message to list missing names; got %q", findings[0].Message)
	}
}

func TestAbstractMemberNotImplemented_MissingAbstractMethod_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "AbstractMemberNotImplemented", `
package test

abstract class Base {
    abstract fun handle()
}

class Impl : Base()
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when class fails to implement abstract base method")
	}
}

func TestAbstractMemberNotImplemented_AllImplemented_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "AbstractMemberNotImplemented", `
package test

interface Greeter {
    fun greet(name: String): String
    fun farewell()
}

class FullGreeter : Greeter {
    override fun greet(name: String): String = "Hi $name"
    override fun farewell() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when all members implemented, got %d", len(findings))
	}
}

func TestAbstractMemberNotImplemented_PrimaryConstructorOverride_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "AbstractMemberNotImplemented", `
package test

interface Named {
    val name: String
}

class NamedThing(override val name: String) : Named
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when constructor override val provides member, got %d", len(findings))
	}
}

func TestAbstractMemberNotImplemented_AbstractClassExempt_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "AbstractMemberNotImplemented", `
package test

interface Greeter {
    fun greet(name: String): String
}

abstract class StillAbstract : Greeter
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings on abstract subclass, got %d", len(findings))
	}
}

func TestAbstractMemberNotImplemented_NoSupertypes_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "AbstractMemberNotImplemented", `
package test

class Standalone {
    fun foo() = 42
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings on class without supertypes, got %d", len(findings))
	}
}

func TestAbstractMemberNotImplemented_TransitiveInterfaceMethod_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "AbstractMemberNotImplemented", `
package test

interface Greeter {
    fun greet(name: String): String
}

abstract class GreeterBase : Greeter

class HollowChild : GreeterBase()
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when interface member missing two levels deep")
	}
	if !strings.Contains(findings[0].Message, "greet") {
		t.Errorf("expected message to mention transitive missing 'greet'; got %q", findings[0].Message)
	}
}

func TestAbstractMemberNotImplemented_TransitiveImplemented_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "AbstractMemberNotImplemented", `
package test

interface Greeter {
    fun greet(name: String): String
}

abstract class GreeterBase : Greeter {
    override fun greet(name: String): String = "hi $name"
}

class FullChild : GreeterBase()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when intermediate concrete impl supplies the member, got %d", len(findings))
	}
}
