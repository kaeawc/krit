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
    fun provideB(): Handler2 = HandlerB()
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
