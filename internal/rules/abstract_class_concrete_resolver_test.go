package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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

func runAbstractClassConcreteWithSourceResolver(t *testing.T, code string) int {
	t.Helper()
	file := writeKotlinFile(t, code, "Prefs.kt")
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	cols := NewDispatcher([]*api.Rule{abstractClassConcreteRule(t)}, resolver).Run(file)
	return cols.Len()
}

// An interface member without a body is abstract, so an abstract class that leaves it unimplemented stays abstract.
func TestAbstractClassCanBeConcreteClassKeepsClassWithUnimplementedInterfaceMember(t *testing.T) {
	n := runAbstractClassConcreteWithSourceResolver(t, `package p

interface Prefs {
    val storage: String
    fun reset()
}

abstract class BasePrefs(override val storage: String) : Prefs
`)
	if n != 0 {
		t.Fatalf("got %d finding(s) while reset() is unimplemented, want none", n)
	}
}

func TestAbstractClassCanBeConcreteClassReportsWhenInterfaceMembersAreImplemented(t *testing.T) {
	n := runAbstractClassConcreteWithSourceResolver(t, `package p

interface Prefs {
    val storage: String
    fun reset()
}

abstract class BasePrefs(override val storage: String) : Prefs {
    override fun reset() {}
}
`)
	if n != 1 {
		t.Fatalf("got %d finding(s), want 1", n)
	}
}

func runAbstractRuleAcrossFiles(t *testing.T, ruleID, target string, sources map[string]string) int {
	t.Helper()
	var files []*scanner.File
	var targetFile *scanner.File
	for name, code := range sources {
		f := writeKotlinFile(t, code, name)
		files = append(files, f)
		if name == target {
			targetFile = f
		}
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel(files, 1)
	for _, r := range api.Registry {
		if r.ID == ruleID {
			cols := NewDispatcher([]*api.Rule{r}, resolver).Run(targetFile)
			return cols.Len()
		}
	}
	t.Fatalf("%s is not registered", ruleID)
	return 0
}

func TestAbstractClassRulesSkipClassWithSubclassInAnotherFile(t *testing.T) {
	sources := map[string]string{
		"Base.kt":    "package p\n\nabstract class Base {\n    fun shared() = 1\n}\n",
		"Derived.kt": "package p\n\nclass Derived : Base()\n",
	}
	for _, id := range []string{"AbstractClassCanBeConcreteClass", "AbstractClassCanBeInterface"} {
		if n := runAbstractRuleAcrossFiles(t, id, "Base.kt", sources); n != 0 {
			t.Errorf("%s: got %d finding(s) for a base class that has a subclass, want none", id, n)
		}
	}
}

func TestAbstractClassCanBeConcreteClassStillFiresWithoutSubclass(t *testing.T) {
	sources := map[string]string{"Base.kt": "package p\n\nabstract class Base {\n    fun shared() = 1\n}\n"}
	if n := runAbstractRuleAcrossFiles(t, "AbstractClassCanBeConcreteClass", "Base.kt", sources); n != 1 {
		t.Fatalf("got %d finding(s), want 1", n)
	}
}

// An abstract member declared two levels up, through a sealed class in another file, keeps the class abstract.
func TestAbstractClassCanBeConcreteClassCountsAbstractMemberFromGrandparent(t *testing.T) {
	sources := map[string]string{
		"Renderable.kt": "package p.model\n\ninterface Renderable {\n    fun draw()\n}\n",
		"Mid.kt":        "package p.model\n\nsealed class MidElement : Renderable {\n    val tag = \"x\"\n}\n",
		"Leaf.kt":       "package p.ui\n\nimport p.model.MidElement\n\nabstract class LeafElement : MidElement() {\n    fun extra() = 1\n}\n",
	}
	if n := runAbstractRuleAcrossFiles(t, "AbstractClassCanBeConcreteClass", "Leaf.kt", sources); n != 0 {
		t.Fatalf("got %d finding(s) while draw() is inherited unimplemented, want none", n)
	}
}

func TestAbstractClassCanBeConcreteClassReportsWhenGrandparentMemberIsImplemented(t *testing.T) {
	sources := map[string]string{
		"Renderable.kt": "package p.model\n\ninterface Renderable {\n    fun draw()\n}\n",
		"Mid.kt":        "package p.model\n\nsealed class MidElement : Renderable {\n    val tag = \"x\"\n}\n",
		"Leaf.kt":       "package p.ui\n\nimport p.model.MidElement\n\nabstract class LeafElement : MidElement() {\n    override fun draw() {}\n}\n",
	}
	if n := runAbstractRuleAcrossFiles(t, "AbstractClassCanBeConcreteClass", "Leaf.kt", sources); n != 1 {
		t.Fatalf("got %d finding(s), want 1", n)
	}
}
