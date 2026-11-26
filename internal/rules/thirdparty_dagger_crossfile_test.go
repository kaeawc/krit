package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
