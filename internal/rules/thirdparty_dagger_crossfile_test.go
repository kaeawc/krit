package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const daggerProvidersModule = `package com.example

import dagger.Module
import dagger.Provides
import dagger.producers.Produces

@Module
object NetModule {
  @Provides fun provideClient(): String = "client"
  @Produces fun produceToken(): String = "token"
}
`

// runDoNotCallProviders runs DoNotCallProviders over the sources, written under a temporary project.
func runDoNotCallProviders(t *testing.T, sources map[string]string) []string {
	t.Helper()
	return runCrossFileByID(t, "DoNotCallProviders", sources)
}

// As in Lint, which runs the detector on main sources only, a call from a test source is not reported; a
// @Produces method counts like a @Provides one.
func TestDoNotCallProvidersSkipsTestSources(t *testing.T) {
	caller := "package com.example\n\nfun use() {\n  NetModule.provideClient()\n  NetModule.produceToken()\n}\n"
	got := runDoNotCallProviders(t, map[string]string{
		"src/main/java/com/example/NetModule.kt": daggerProvidersModule,
		"src/main/java/com/example/Use.kt":       caller,
		"src/test/java/com/example/UseTest.kt":   caller,
	})
	want := []string{"src/main/java/com/example/Use.kt:4", "src/main/java/com/example/Use.kt:5"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("want %v, got %v", want, got)
	}
}

// A call inside a class annotated javax.annotation.processing.Generated is Dagger's own and is not reported; a
// same-named method of another class, or one annotated with a lookalike @Provides, is not a provider.
func TestDoNotCallProvidersNegatives(t *testing.T) {
	got := runDoNotCallProviders(t, map[string]string{
		"src/main/java/com/example/NetModule.kt": daggerProvidersModule,
		"src/main/java/com/example/Factory.java": `package com.example;

@javax.annotation.processing.Generated("dagger.internal.codegen.ComponentProcessor")
final class NetModule_ProvideClientFactory {
  String get() { return NetModule.INSTANCE.provideClient(); }
}
`,
		"src/main/java/com/example/Other.kt": `package com.example

annotation class Provides

class Cache {
  @Provides fun provideClient(): String = "cached"
}

fun use(cache: Cache) {
  cache.provideClient()
  with(cache) { provideClient() }
}
`,
	})
	if len(got) != 0 {
		t.Errorf("want no findings, got %v", got)
	}
}

func runBindsTypeMismatch(t *testing.T, src string) []string {
	t.Helper()
	return runCrossFileByID(t, "BindsTypeMismatch", map[string]string{"src/main/java/com/example/Binds.kt": src})
}

// runCrossFileByID writes the sources under a temporary project and runs the cross-file rule id over all of them,
// returning "path:line" for each finding, sorted.
func runCrossFileByID(t *testing.T, id string, sources map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	var files []*scanner.File
	for name, src := range sources {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		parse := scanner.ParseFile
		if filepath.Ext(path) == ".java" {
			parse = scanner.ParseJavaFile
		}
		file, err := parse(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	for _, r := range api.Registry {
		if r.ID == id {
			var out []string
			for _, f := range runCrossFileRuleOnFiles(r, files...) {
				rel, _ := filepath.Rel(dir, f.File)
				out = append(out, filepath.ToSlash(rel)+":"+strconv.Itoa(f.Line))
			}
			sort.Strings(out)
			return out
		}
	}
	t.Fatalf("rule %s not registered", id)
	return nil
}

// A @Binds method the detector already reported (here: not in a module) gets no BindsTypeMismatch; nor does one
// whose return type is parameterized, a library type, or whose parameter type the source does not show.
func TestBindsTypeMismatchAbstains(t *testing.T) {
	const src = `package com.example

import dagger.Binds
import dagger.Module

interface Repo
interface Holder<T>
class Impl

interface NotAModule {
  @Binds fun bindOutside(impl: Impl): Repo
}

@Module
interface RepoModule {
  @Binds fun bindGeneric(impl: Impl): Holder<String>
  @Binds fun bindLibrary(impl: Impl): CharSequence
  @Binds fun bindUnknown(impl: Mystery): Repo
  @Binds fun bindWrong(impl: Impl): Repo
}
`
	if got := runBindsTypeMismatch(t, src); len(got) != 1 || got[0] != "src/main/java/com/example/Binds.kt:19" {
		t.Errorf("want one finding, on bindWrong (line 19), got %v", got)
	}
}

// Supertypes are followed through other project classes, including an interface's own supertypes.
func TestBindsTypeMismatchFollowsSupertypes(t *testing.T) {
	const src = `package com.example

import dagger.Binds
import dagger.Module

interface Repo
interface LocalRepo : Repo
abstract class BaseRepo : LocalRepo
class RepoImpl : BaseRepo()

@Module
interface RepoModule {
  @Binds fun bindRepo(impl: RepoImpl): Repo
}
`
	if got := runBindsTypeMismatch(t, src); len(got) != 0 {
		t.Errorf("want no findings, got %v", got)
	}
}

// Orchestrator first pass on #1841: a provider with parameters is matched by its arity (and a plain overload
// with the call's arity is not the provider, in Java too); a nested sibling type and a typealias do not make a
// valid @Binds a mismatch; a call in a Java anonymous class inside a @Generated class is reported.
func TestDaggerCrossFileFirstPassReview(t *testing.T) {
	module := `package com.example

import dagger.Module
import dagger.Provides

@Module
object NetModule {
  @Provides fun provideClient(url: String): String = url
  @Provides fun provideTwo(a: Int, b: Int): Int = a + b
  fun provideTwo(a: Int): Int = a
}
`
	got := runCrossFileByID(t, "DoNotCallProviders", map[string]string{
		"src/main/java/com/example/NetModule.kt": module,
		"src/main/java/com/example/Use.kt":       "package com.example\n\nfun use() {\n  NetModule.provideClient(\"x\")\n  NetModule.provideTwo(1)\n}\n",
		"src/main/java/com/example/UseJ.java": `package com.example;

class UseJ {
  void use() {
    NetModule.INSTANCE.provideTwo(1);
    NetModule.INSTANCE.provideTwo(1, 2);
  }
}
`,
		"src/main/java/com/example/Gen.java": `package com.example;

@javax.annotation.processing.Generated("x")
final class Gen {
  Runnable r = new Runnable() {
    public void run() { NetModule.INSTANCE.provideClient("y"); }
  };
}
`,
	})
	want := []string{"src/main/java/com/example/Gen.java:6", "src/main/java/com/example/Use.kt:4", "src/main/java/com/example/UseJ.java:6"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("DoNotCallProviders: got %v, want %v", got, want)
	}

	binds := `package com.example

import dagger.Binds
import dagger.Module
import com.example.aliases.ImplAlias

interface Feature {
  interface Repo
  class Impl : Repo
}

interface Repo

typealias RepoAlias = Repo

class ViaAlias : RepoAlias

@Module
interface Bindings {
  @Binds fun nested(impl: Feature.Impl): Feature.Repo
  @Binds fun alias(impl: ImplAlias): Repo
  @Binds fun via(impl: ViaAlias): Repo
}
`
	got = runCrossFileByID(t, "BindsTypeMismatch", map[string]string{
		"src/main/java/com/example/Bindings.kt":        binds,
		"src/main/java/com/example/aliases/Aliases.kt": "package com.example.aliases\n\nimport com.example.Repo\n\nclass RealImpl : Repo\n\ntypealias ImplAlias = RealImpl\n",
		"src/main/java/com/example/JOuter.java": `package com.example;

import dagger.Binds;
import dagger.Module;

interface JRepo {}

class JOuter {
  static class JImpl implements JRepo {}
}

@Module
abstract class JBindings {
  @Binds abstract JRepo bind(JOuter.JImpl impl);
}
`,
	})
	if len(got) != 0 {
		t.Errorf("BindsTypeMismatch: want no findings, got %v", got)
	}
}

// #1867: an overload in another file of the package, an overload or override in a subclass, a local function and a
// function-typed parameter each take the call away from the @MustUseNamedParams declaration.
func TestMustUseNamedParamsOverloadsAcrossFiles(t *testing.T) {
	got := runCrossFileByID(t, "MustUseNamedParams", map[string]string{
		"src/main/java/com/example/Api.kt": `package com.example

import slack.lint.annotations.MustUseNamedParams

@MustUseNamedParams
fun overloaded(a: Int, b: Int) = a + b

open class Base {
  @MustUseNamedParams
  open fun foo(a: Int, b: Int) = a
}

open class OBase {
  @MustUseNamedParams
  open fun bar(a: Int, b: Int) = a
}
`,
		"src/main/java/com/example/Api2.kt": "package com.example\n\nfun overloaded(s: String, t: String) = s + t\n",
		"src/main/java/com/example/Sub.kt": `package com.example

class Sub : Base() {
  fun foo(s: String, t: String) = s
}

class OSub : OBase() {
  override fun bar(a: Int, b: Int) = b
}

class Local : Base() {
  fun m(): Int {
    fun foo(a: Int, b: Int) = a
    return foo(1, 2)
  }
}

fun calls(s: Sub, o: OSub, b: Base, p: (Int, Int) -> Unit) {
  overloaded("x", "y")
  s.foo("a", "b")
  o.bar(1, 2)
  b.foo(1, 2)
}

fun shadowed(overloaded: (Int, Int) -> Unit) {
  overloaded(1, 2)
}
`,
	})
	want := []string{"src/main/java/com/example/Sub.kt:22"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got %v, want %v (only b.foo(1, 2))", got, want)
	}
}
