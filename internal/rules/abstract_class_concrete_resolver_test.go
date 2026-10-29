package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// oracleShellComposite answers ClassHierarchy the way the oracle-backed
// composite does when no active rule asked krit-types for members: library
// supertypes resolve, but with an empty member list.
type oracleShellComposite struct {
	compositeStub
	classes map[string]*typeinfer.ClassInfo
}

func (c *oracleShellComposite) ClassHierarchy(name string) *typeinfer.ClassInfo {
	return c.classes[name]
}

func abstractClassConcreteRule(t *testing.T) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == "AbstractClassCanBeConcreteClass" {
			return r
		}
	}
	t.Fatal("AbstractClassCanBeConcreteClass is not registered")
	return nil
}

// #956: a library supertype whose members the oracle did not export must not
// read as "nothing abstract to implement". The rule uses the source resolver,
// which does not know BuildService or LongSupplier, so it stays silent.
func TestAbstractClassCanBeConcreteClassIgnoresOracleClassShells(t *testing.T) {
	file := writeKotlinFile(t, `package p

import java.util.function.LongSupplier
import org.gradle.api.services.BuildService
import org.gradle.api.services.BuildServiceParameters

abstract class RunStart : BuildService<BuildServiceParameters.None>, LongSupplier {
    private val startedAtMs: Long = System.currentTimeMillis()

    override fun getAsLong(): Long = startedAtMs
}
`, "RunStart.kt")
	composite := &oracleShellComposite{
		compositeStub: compositeStub{fallback: typeinfer.NewFakeResolver()},
		classes: map[string]*typeinfer.ClassInfo{
			"RunStart": {Name: "RunStart", FQN: "p.RunStart", Kind: "class", IsAbstract: true,
				Supertypes: []string{"org.gradle.api.services.BuildService", "java.util.function.LongSupplier"}},
			"BuildService": {Name: "BuildService", FQN: "org.gradle.api.services.BuildService", Kind: "interface"},
			"LongSupplier": {Name: "LongSupplier", FQN: "java.util.function.LongSupplier", Kind: "interface"},
		},
	}
	cols := NewDispatcher([]*api.Rule{abstractClassConcreteRule(t)}, composite).Run(file)
	if n := cols.Len(); n != 0 {
		t.Fatalf("got %d finding(s) from oracle class shells, want none", n)
	}
}

// The source resolver still decides: supertypes it knows, with nothing
// abstract left unimplemented, keep the finding.
func TestAbstractClassCanBeConcreteClassStillFiresOnKnownSourceSupertypes(t *testing.T) {
	file := writeKotlinFile(t, `package p

interface Named {
    fun name(): String
}

abstract class Impl : Named {
    override fun name(): String = "x"
}
`, "Impl.kt")
	source := typeinfer.NewFakeResolver()
	source.Classes["Impl"] = &typeinfer.ClassInfo{Name: "Impl", FQN: "p.Impl", Kind: "class", IsAbstract: true, Supertypes: []string{"p.Named"}}
	source.Classes["Named"] = &typeinfer.ClassInfo{Name: "Named", FQN: "p.Named", Kind: "interface",
		Members: []typeinfer.MemberInfo{{Name: "name", Kind: "function", IsAbstract: true}}}
	composite := &oracleShellComposite{compositeStub: compositeStub{fallback: source}}
	cols := NewDispatcher([]*api.Rule{abstractClassConcreteRule(t)}, composite).Run(file)
	if n := cols.Len(); n != 1 {
		t.Fatalf("got %d finding(s), want 1", n)
	}
}
