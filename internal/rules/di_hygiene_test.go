package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestBindsInsteadOfProvides(t *testing.T) {
	rule := buildRuleIndex()["BindsInsteadOfProvides"]
	if rule == nil {
		t.Fatal("BindsInsteadOfProvides rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "BindsInsteadOfProvides.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "BindsInsteadOfProvides.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "provideFoo") {
			t.Fatalf("unexpected message: %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestBindsInsteadOfProvides_MessageDependsOnModuleKind(t *testing.T) {
	const decls = `
package test
annotation class Module
annotation class Provides
annotation class Singleton
annotation class Named
interface Foo
class FooImpl : Foo
`
	tests := []struct {
		name       string
		module     string
		wantMove   bool
		wantFinder int
	}{
		{"object module", "@Module object M { @Provides fun provideFoo(impl: FooImpl): Foo = impl }", true, 0},
		{"concrete class module", "@Module class M { @Provides fun provideFoo(impl: FooImpl): Foo = impl }", true, 0},
		{"scoped provider", "@Module abstract class M { @Provides @Singleton fun provideFoo(impl: FooImpl): Foo = impl }", false, 0},
		{"qualifier only on provider", "@Module abstract class M { @Provides @Named fun provideFoo(impl: FooImpl): Foo = impl }", false, 0},
		{"qualifier only on param", "@Module abstract class M { @Provides fun provideFoo(@Named impl: FooImpl): Foo = impl }", false, 0},
		{"same qualifier on both", "@Module abstract class M { @Provides @Named fun provideFoo(@Named impl: FooImpl): Foo = impl }", false, 1},
		{"abstract class module", "@Module abstract class M { @Provides fun provideFoo(impl: FooImpl): Foo = impl }", false, 1},
		{"interface module", "@Module interface M { @Provides fun provideFoo(impl: FooImpl): Foo = impl }", false, 1},
		{"companion of abstract module", "@Module abstract class M {\n    companion object {\n        @Provides fun provideFoo(impl: FooImpl): Foo = impl\n    }\n}", true, 1},
		{"companion of interface module", "@Module interface M {\n    companion object {\n        @Provides fun provideFoo(impl: FooImpl): Foo = impl\n    }\n}", true, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := runRuleByName(t, "BindsInsteadOfProvides", decls+tt.module)
			if len(findings) != tt.wantFinder {
				t.Fatalf("expected %d finding, got %d", tt.wantFinder, len(findings))
			}
			if len(findings) == 0 {
				return
			}
			gotMove := strings.Contains(findings[0].Message, "move it to an abstract @Binds member")
			if gotMove != tt.wantMove {
				t.Fatalf("move message = %v, want %v: %q", gotMove, tt.wantMove, findings[0].Message)
			}
			if !tt.wantMove && !strings.Contains(findings[0].Message, "replace with @Binds") {
				t.Fatalf("expected replace-with-@Binds message, got %q", findings[0].Message)
			}
		})
	}
}

func TestBindsReturnTypeMatchesParam(t *testing.T) {
	rule := buildRuleIndex()["BindsReturnTypeMatchesParam"]
	if rule == nil {
		t.Fatal("BindsReturnTypeMatchesParam rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "BindsReturnTypeMatchesParam.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "BindsReturnTypeMatchesParam.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "Foo") {
			t.Fatalf("unexpected message: %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestComponentMissingModule(t *testing.T) {
	rule := buildRuleIndex()["ComponentMissingModule"]
	if rule == nil {
		t.Fatal("ComponentMissingModule rule not registered")
	}
	if !rule.Needs.Has(api.NeedsCrossFile) {
		t.Fatal("ComponentMissingModule does not declare NeedsCrossFile")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "ComponentMissingModule.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "ComponentMissingModule.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "BModule") {
			t.Fatalf("unexpected message: %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
		}
	})
}

func TestBindsMismatchedArity(t *testing.T) {
	rule := buildRuleIndex()["BindsMismatchedArity"]
	if rule == nil {
		t.Fatal("BindsMismatchedArity rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "BindsMismatchedArity.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "BindsMismatchedArity.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestAnvilContributesBindingWithoutScope(t *testing.T) {
	rule := buildRuleIndex()["AnvilContributesBindingWithoutScope"]
	if rule == nil {
		t.Fatal("AnvilContributesBindingWithoutScope rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "AnvilContributesBindingWithoutScope.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "AnvilContributesBindingWithoutScope.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func runCrossFileRule(rule *api.Rule, files []*scanner.File) []scanner.Finding {
	idx := scanner.BuildIndex(files, 1)
	ctx := &api.Context{
		CodeIndex: idx,
		Collector: scanner.NewFindingCollector(0),
	}
	rule.Check(ctx)
	cols := *ctx.Collector.Columns()
	out := make([]scanner.Finding, cols.Len())
	for i := range out {
		out[i] = cols.Finding(i)
	}
	return out
}

func TestAnvilMergeComponentEmptyScope(t *testing.T) {
	rule := buildRuleIndex()["AnvilMergeComponentEmptyScope"]
	if rule == nil {
		t.Fatal("AnvilMergeComponentEmptyScope rule not registered")
	}

	if !rule.Needs.Has(api.NeedsCrossFile) {
		t.Fatal("AnvilMergeComponentEmptyScope does not declare NeedsCrossFile")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "AnvilMergeComponentEmptyScope.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "AnvilMergeComponentEmptyScope.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}

		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "UnusedScope::class") {
			t.Fatalf("expected scope in message, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}

		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("cross-file contributes-to satisfies scope", func(t *testing.T) {
		files := parseKotlinFiles(t,
			"Component.kt", `package dihygiene

import kotlin.reflect.KClass

annotation class MergeComponent(val scope: KClass<*>)

object AppScope

@MergeComponent(AppScope::class)
interface AppComponent
`,
			"Contribution.kt", `package dihygiene

import kotlin.reflect.KClass

annotation class ContributesTo(val scope: KClass<*>)

object AppScope

@ContributesTo(AppScope::class)
interface AppApi
`,
		)

		findings := runCrossFileRule(rule, files)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("cross-file contributes-binding satisfies scope", func(t *testing.T) {
		files := parseKotlinFiles(t,
			"Component.kt", `package dihygiene

import kotlin.reflect.KClass

annotation class MergeComponent(val scope: KClass<*>)

object AppScope

@MergeComponent(AppScope::class)
interface AppComponent
`,
			"Binding.kt", `package dihygiene

import kotlin.reflect.KClass

annotation class ContributesBinding(val scope: KClass<*>)

interface AppApi
object AppScope

@ContributesBinding(AppScope::class)
class AppApiImpl : AppApi
`,
		)

		findings := runCrossFileRule(rule, files)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestDeadBindings(t *testing.T) {
	rule := buildRuleIndex()["DeadBindings"]
	if rule == nil {
		t.Fatal("DeadBindings rule not registered")
	}

	if !rule.Needs.Has(api.NeedsCrossFile) {
		t.Fatal("DeadBindings does not declare NeedsCrossFile")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "DeadBindings.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "DeadBindings.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "DeadApi") {
			t.Fatalf("expected dead binding's return type in message, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("cross-file inject demand satisfies binding", func(t *testing.T) {
		files := parseKotlinFiles(t,
			"Module.kt", `package dihygiene

annotation class Module
annotation class Provides
annotation class Inject

interface Foo
class FooImpl : Foo

@Module
object FooModule {
    @Provides
    fun provideFoo(): Foo = FooImpl()
}
`,
			"Consumer.kt", `package dihygiene

class Consumer @Inject constructor(
    private val foo: Foo,
)
`,
		)

		findings := runCrossFileRule(rule, files)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestHiltInstallInMismatch(t *testing.T) {
	rule := buildRuleIndex()["HiltInstallInMismatch"]
	if rule == nil {
		t.Fatal("HiltInstallInMismatch rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "HiltInstallInMismatch.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "HiltInstallInMismatch.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "ActivityScoped") {
			t.Fatalf("expected scope in message, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestSubcomponentNotInstalled(t *testing.T) {
	rule := buildRuleIndex()["SubcomponentNotInstalled"]
	if rule == nil {
		t.Fatal("SubcomponentNotInstalled rule not registered")
	}
	if !rule.Needs.Has(api.NeedsCrossFile) {
		t.Fatal("SubcomponentNotInstalled does not declare NeedsCrossFile")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "SubcomponentNotInstalled.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "SubcomponentNotInstalled.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "UserSubcomponent") {
			t.Fatalf("unexpected message: %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("cross-file install satisfies", func(t *testing.T) {
		files := parseKotlinFiles(t,
			"Sub.kt", `package dihygiene

annotation class Subcomponent

@Subcomponent
interface UserSubcomponent {
    interface Factory { fun create(): UserSubcomponent }
}
`,
			"App.kt", `package dihygiene

annotation class Component

@Component
interface AppComponent {
    fun userSub(): UserSubcomponent.Factory
}
`,
		)
		findings := runCrossFileRule(rule, files)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestIntoMapDuplicateKey(t *testing.T) {
	rule := buildRuleIndex()["IntoMapDuplicateKey"]
	if rule == nil {
		t.Fatal("IntoMapDuplicateKey rule not registered")
	}
	if !rule.Needs.Has(api.NeedsCrossFile) {
		t.Fatal("IntoMapDuplicateKey does not declare NeedsCrossFile")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "IntoMapDuplicateKey.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "IntoMapDuplicateKey.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("cross-file duplicates flagged", func(t *testing.T) {
		files := parseKotlinFiles(t,
			"FileA.kt", `package dihygiene

annotation class Module
annotation class Provides
annotation class IntoMap
annotation class StringKey(val value: String)

interface Handler
class HandlerA : Handler

@Module
object HandlerModule {
    @Provides @IntoMap @StringKey("foo")
    fun provideA(): Handler = HandlerA()
}
`,
			"FileB.kt", `package dihygiene

annotation class Module2
annotation class Provides2
annotation class IntoMap2
annotation class StringKey2(val value: String)

interface Handler2
class HandlerB : Handler2

@Module2
object HandlerModule {
    @Provides @IntoMap @StringKey("foo")
    fun provideB(): Handler = HandlerB()
}
`,
		)
		findings := runCrossFileRule(rule, files)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})
}

func TestIntoSetDuplicateType(t *testing.T) {
	rule := buildRuleIndex()["IntoSetDuplicateType"]
	if rule == nil {
		t.Fatal("IntoSetDuplicateType rule not registered")
	}
	if !rule.Needs.Has(api.NeedsCrossFile) {
		t.Fatal("IntoSetDuplicateType does not declare NeedsCrossFile")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "IntoSetDuplicateType.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "IntoSetDuplicateType.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestProviderInsteadOfLazy(t *testing.T) {
	rule := buildRuleIndex()["ProviderInsteadOfLazy"]
	if rule == nil {
		t.Fatal("ProviderInsteadOfLazy rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "ProviderInsteadOfLazy.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "ProviderInsteadOfLazy.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "api") {
			t.Fatalf("expected param name in message, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestLazyInsteadOfDirect(t *testing.T) {
	rule := buildRuleIndex()["LazyInsteadOfDirect"]
	if rule == nil {
		t.Fatal("LazyInsteadOfDirect rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "LazyInsteadOfDirect.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "LazyInsteadOfDirect.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestHiltSingletonWithActivityDep(t *testing.T) {
	rule := buildRuleIndex()["HiltSingletonWithActivityDep"]
	if rule == nil {
		t.Fatal("HiltSingletonWithActivityDep rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "HiltSingletonWithActivityDep.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "HiltSingletonWithActivityDep.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "NavigatorImpl") || !strings.Contains(findings[0].Message, "Activity") {
			t.Fatalf("unexpected message: %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestInjectOnAbstractClass(t *testing.T) {
	rule := buildRuleIndex()["InjectOnAbstractClass"]
	if rule == nil {
		t.Fatal("InjectOnAbstractClass rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "InjectOnAbstractClass.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "InjectOnAbstractClass.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "BaseUseCase") {
			t.Fatalf("expected class name in message, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestSingletonOnMutableClass(t *testing.T) {
	rule := buildRuleIndex()["SingletonOnMutableClass"]
	if rule == nil {
		t.Fatal("SingletonOnMutableClass rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "SingletonOnMutableClass.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "SingletonOnMutableClass.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 findings, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestMetroFactoryDeclarationShape(t *testing.T) {
	rule := buildRuleIndex()["MetroFactoryDeclarationShape"]
	if rule == nil {
		t.Fatal("MetroFactoryDeclarationShape rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "MetroFactoryDeclarationShape.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "MetroFactoryDeclarationShape.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestScopeOnParameterizedClass(t *testing.T) {
	rule := buildRuleIndex()["ScopeOnParameterizedClass"]
	if rule == nil {
		t.Fatal("ScopeOnParameterizedClass rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "ScopeOnParameterizedClass.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "ScopeOnParameterizedClass.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "Cache") {
			t.Fatalf("expected class name in message, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestMissingJvmSuppressWildcards(t *testing.T) {
	rule := buildRuleIndex()["MissingJvmSuppressWildcards"]
	if rule == nil {
		t.Fatal("MissingJvmSuppressWildcards rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "MissingJvmSuppressWildcards.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "MissingJvmSuppressWildcards.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestMissingJvmSuppressWildcards_MetroFileSkipped(t *testing.T) {
	const body = `
@Provides
fun provideAll(items: Set<Plugin>): Registry = Registry(items)
`
	if got := runRuleByName(t, "MissingJvmSuppressWildcards", "package test\nimport dagger.Provides\n"+body); len(got) != 1 {
		t.Fatalf("expected 1 finding without Metro, got %d: %v", len(got), got)
	}
	if got := runRuleByName(t, "MissingJvmSuppressWildcards", "package test\nimport dev.zacsweers.metro.Provides\n"+body); len(got) != 0 {
		t.Fatalf("expected no findings in a Metro file, got %d: %v", len(got), got)
	}
}

func TestModuleWithNonStaticProvides(t *testing.T) {
	rule := buildRuleIndex()["ModuleWithNonStaticProvides"]
	if rule == nil {
		t.Fatal("ModuleWithNonStaticProvides rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "ModuleWithNonStaticProvides.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "ModuleWithNonStaticProvides.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "provideB") {
			t.Fatalf("expected provider name in message, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestIntoMapMissingKey(t *testing.T) {
	rule := buildRuleIndex()["IntoMapMissingKey"]
	if rule == nil {
		t.Fatal("IntoMapMissingKey rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "IntoMapMissingKey.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "IntoMapMissingKey.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "@*Key") {
			t.Fatalf("expected key annotation in message, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestIntoSetOnNonSetReturn(t *testing.T) {
	rule := buildRuleIndex()["IntoSetOnNonSetReturn"]
	if rule == nil {
		t.Fatal("IntoSetOnNonSetReturn rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "IntoSetOnNonSetReturn.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "IntoSetOnNonSetReturn.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "List<Plugin>") {
			t.Fatalf("expected return type in message, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestHiltEntryPointOnNonInterface(t *testing.T) {
	rule := buildRuleIndex()["HiltEntryPointOnNonInterface"]
	if rule == nil {
		t.Fatal("HiltEntryPointOnNonInterface rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "HiltEntryPointOnNonInterface.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "HiltEntryPointOnNonInterface.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(context.Background(), negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func parseKotlinFiles(t *testing.T, filesAndContents ...string) []*scanner.File {
	t.Helper()
	if len(filesAndContents)%2 != 0 {
		t.Fatal("parseKotlinFiles requires path/content pairs")
	}

	dir := t.TempDir()
	parsed := make([]*scanner.File, 0, len(filesAndContents)/2)
	for i := 0; i < len(filesAndContents); i += 2 {
		path := filepath.Join(dir, filesAndContents[i])
		if err := os.WriteFile(path, []byte(filesAndContents[i+1]), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", path, err)
		}
		parsed = append(parsed, file)
	}
	return parsed
}

func TestBindsMismatchedArity_ExactAnnotationNames(t *testing.T) {
	findings := runRuleByName(t, "BindsMismatchedArity", `
package test

@Module
interface FeatureModule {
    @BindsOptionalOf
    fun optionalThing(): Thing

    @Binds
    fun bindImpl(impl: ThingImpl): Thing

    @Binds
    fun ThingImpl.bindReceiver(): Thing
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestBindsMismatchedArity_FlagsWrongArityPerAnnotation(t *testing.T) {
	findings := runRuleByName(t, "BindsMismatchedArity", `
package test

@Module
interface FeatureModule {
    @BindsOptionalOf
    fun optionalThing(extra: Int): Thing

    @Binds
    fun bindNothing(): Thing
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
}

func TestSingletonOnMutableClass_ProtectedVarsNotFlagged(t *testing.T) {
	findings := runRuleByName(t, "SingletonOnMutableClass", `
package test

import javax.inject.Singleton

@Singleton
class Guarded {
    @Volatile private var a = 0
    @GuardedBy("lock") private var b = 0
    private var c by atomic(0)
    private var d = AtomicInteger(0)
    private var e = 0

    @Synchronized fun bump() { e++ }
    @Synchronized fun read(): Int = e
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestSingletonOnMutableClass_UnprotectedVarStillFlagged(t *testing.T) {
	findings := runRuleByName(t, "SingletonOnMutableClass", `
package test

import javax.inject.Singleton

@Singleton
class Unguarded {
    private var e = 0

    fun bump() { e++ }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestMissingJvmSuppressWildcards_TypeUseAnnotationAccepted(t *testing.T) {
	findings := runRuleByName(t, "MissingJvmSuppressWildcards", `
package test
import dagger.Provides
class M {
    @Provides fun a(x: @JvmSuppressWildcards Set<Foo>): Bar = Bar()
    @Provides fun b(x: Set<@JvmSuppressWildcards Foo>): Bar = Bar()
    @Provides fun c(x: @JvmSuppressWildcards Map<String, Foo>?): Bar = Bar()
    @Provides fun d(x: Set<Foo>): Bar = Bar()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected only the unannotated parameter to be flagged, got %d: %v", len(findings), findings)
	}
}

func TestSingletonOnMutableClass_OneFindingListsAllProperties(t *testing.T) {
	findings := runRuleByName(t, "SingletonOnMutableClass", `
package test

import javax.inject.Singleton

@Singleton
class Many {
    private var a = 0
    private var b = 0
    val c = mutableListOf<Int>()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	for _, want := range []string{"'a' (var property)", "'b' (var property)", "'c' (val initialised by mutableListOf())"} {
		if !strings.Contains(findings[0].Message, want) {
			t.Fatalf("message missing %q: %s", want, findings[0].Message)
		}
	}
}

func TestSingletonOnMutableClass_ThreadConfinedSkipped(t *testing.T) {
	cases := map[string]string{
		"handler param": `
@Singleton
class A(private val mainThreadHandler: Handler) {
    private var e = 0
    fun bump() { e++ }
}`,
		"typed dispatcher": `
@Singleton
class B(private val d: MainCoroutineDispatcher) {
    private var e = 0
    fun bump() { e++ }
}`,
		"annotated mutators": `
@Singleton
class C {
    private var e = 0
    @MainThread fun bump() { e++ }
    @MainThread fun read(): Int = e
}`,
	}
	for name, body := range cases {
		if f := runRuleByName(t, "SingletonOnMutableClass", "package test\nimport javax.inject.Singleton\n"+body); len(f) != 0 {
			t.Errorf("%s: expected no findings, got %d", name, len(f))
		}
	}
	f := runRuleByName(t, "SingletonOnMutableClass", `
package test
import javax.inject.Singleton
@Singleton
class D {
    private var e = 0
    @MainThread fun bump() { e++ }
    fun other() { e = 0 }
}`)
	if len(f) != 1 {
		t.Fatalf("partially annotated: expected 1 finding, got %d", len(f))
	}
}

func TestSingletonOnMutableClass_LateinitNeverAssignedSkipped(t *testing.T) {
	findings := runRuleByName(t, "SingletonOnMutableClass", `
package test

import javax.inject.Singleton

@Singleton
class External {
    lateinit var provider: Provider
    fun use() = provider.get()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestSingletonOnMutableClass_LateinitAssignedInClassFlagged(t *testing.T) {
	findings := runRuleByName(t, "SingletonOnMutableClass", `
package test

import javax.inject.Singleton

@Singleton
class Internal {
    lateinit var provider: Provider
    fun reset(p: Provider) { this.provider = p }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestSingletonOnMutableClass_InterfaceAbstractAndSynchronizedAccessorsAreQuiet(t *testing.T) {
	findings := runRuleByName(t, "SingletonOnMutableClass", `
package test
@Singleton interface Contract { var q: Int }
@Singleton abstract class Base @Inject constructor() { abstract var shape: Int }
@Singleton class Guarded @Inject constructor() {
    @get:Synchronized @set:Synchronized var value = 1
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %v", findings)
	}
}

func TestMissingJvmSuppressWildcards_FinalValueTypesAndNestedWrappers(t *testing.T) {
	findings := runRuleByName(t, "MissingJvmSuppressWildcards", `
package test
class FinalImpl
interface Contract
@Module
object M {
    @Provides fun strings(s: Set<String>): Int = 1
    @Provides fun counts(m: Map<String, Int>): Int = 1
    @Provides fun finalClass(s: Set<FinalImpl>): Int = 1
    @Provides fun iface(s: Set<Contract>): Int = 1
    @Provides fun lazyIface(s: Lazy<Set<Contract>>): Int = 1
    @Provides fun suppressed(s: Lazy<Set<@JvmSuppressWildcards Contract>>): Int = 1
    @Provides fun mapIface(m: Map<String, () -> Unit>): Int = 1
}
`)
	if len(findings) != 3 {
		t.Errorf("expected iface, lazyIface and mapIface only, got %v", findings)
	}
}

func TestBindsInsteadOfProvides_NullableAndKotlinInjectAreQuiet(t *testing.T) {
	nullable := runRuleByName(t, "BindsInsteadOfProvides", `
package test
@Module
interface M {
    @Provides fun provNullable(impl: Impl?): Iface? = impl
}
`)
	if len(nullable) != 0 {
		t.Errorf("expected no findings for nullable pass-through, got %v", nullable)
	}
	inject := runRuleByName(t, "BindsInsteadOfProvides", `
package test
import me.tatarka.inject.annotations.Provides
interface Component {
    @Provides fun provPass(impl: Impl): Iface = impl
}
`)
	if len(inject) != 0 {
		t.Errorf("expected no findings in kotlin-inject components, got %v", inject)
	}
}

func TestDeadBindings_WrappedDemandAndMultibindingsAreLive(t *testing.T) {
	rule := buildRuleIndex()["DeadBindings"]
	dir := t.TempDir()
	path := filepath.Join(dir, "Wrapped.kt")
	src := `package test

@Module
object Providers {
    @Provides fun lazyUsed(): LazyUsed = LazyUsed()
    @Provides fun providerUsed(): ProviderUsed = ProviderUsed()
    @Provides @IntoSet fun contribution(): Contribution = Contribution()
    @Provides fun trulyDead(): TrulyDead = TrulyDead()
}

class Consumer @Inject constructor(
    private val a: Lazy<LazyUsed>,
    private val b: Provider<ProviderUsed>,
)
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	findings := runCrossFileRule(rule, []*scanner.File{file})
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "TrulyDead") {
		t.Errorf("expected only TrulyDead to be reported, got %v", findings)
	}
}

func TestComponentMissingModule_IncludesAndAlternativeProvidersCover(t *testing.T) {
	rule := buildRuleIndex()["ComponentMissingModule"]
	dir := t.TempDir()
	path := filepath.Join(dir, "Graph.kt")
	src := `package test

@Module(includes = [IncludedModule::class])
object RootModule {
    @Provides fun root(): Root = Root()
}

@Module
object IncludedModule {
    @Provides fun included(): Included = Included()
}

@Module
object OtherModule {
    @Provides fun alt(): Included = Included()
    @Provides fun lonely(): Lonely = Lonely()
}

@Component(modules = [RootModule::class], dependencies = [Parent::class])
interface AppComponent {
    fun root(): Root
    fun included(): Included
    fun lonely(): Lonely
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	findings := runCrossFileRule(rule, []*scanner.File{file})
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "OtherModule") {
		t.Errorf("expected only the genuinely missing OtherModule, got %v", findings)
	}
}

func TestIntoMapDuplicateKey_DifferentValueTypesAndClassKeys(t *testing.T) {
	rule := buildRuleIndex()["IntoMapDuplicateKey"]
	dir := t.TempDir()
	path := filepath.Join(dir, "Keys.kt")
	src := `package test

@Module
object KeysModule {
    @Provides @IntoMap @StringKey("a") fun first(): Foo = Foo()
    @Provides @IntoMap @StringKey("a") fun second(): Bar = Bar()
    @Provides @IntoMap @ClassKey(Foo::class) fun third(): Base = Foo()
    @Provides @IntoMap @ClassKey(Foo::class) fun fourth(): Base = Foo()
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	findings := runCrossFileRule(rule, []*scanner.File{file})
	if len(findings) != 2 {
		t.Fatalf("expected the two ClassKey duplicates only, got %v", findings)
	}
	for _, f := range findings {
		if !strings.Contains(f.Message, "Foo::class") {
			t.Errorf("message should name the full class key, got %q", f.Message)
		}
	}
}

func TestIntoSetDuplicateType_ComparesWholeExpression(t *testing.T) {
	rule := buildRuleIndex()["IntoSetDuplicateType"]
	dir := t.TempDir()
	path := filepath.Join(dir, "Sets.kt")
	src := `package test

@Module
object SetsModule {
    @Provides @IntoSet fun one(): Plugin = Plugin(1)
    @Provides @IntoSet fun two(): Plugin = Plugin(2)
    @Provides @IntoSet fun three(): Other = Other("x")
    @Provides @IntoSet fun four(): Other = Other("x")
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	findings := runCrossFileRule(rule, []*scanner.File{file})
	if len(findings) != 2 {
		t.Fatalf("expected only the identical Other(\"x\") pair, got %v", findings)
	}
}

func TestHiltInstallInMismatch_CompanionProvidersAreChecked(t *testing.T) {
	findings := runRuleByName(t, "HiltInstallInMismatch", `
package test
@Module
@InstallIn(ActivityComponent::class)
abstract class M3 {
    companion object {
        @Provides @Singleton fun a(): Foo = Foo()
        @Provides @ActivityScoped fun ok(): Foo = Foo()
    }
}
`)
	if len(findings) != 1 {
		t.Errorf("expected the @Singleton provider in the companion, got %v", findings)
	}
}
