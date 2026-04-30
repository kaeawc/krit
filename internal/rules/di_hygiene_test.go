package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestBindsMismatchedArity(t *testing.T) {
	rule := buildRuleIndex()["BindsMismatchedArity"]
	if rule == nil {
		t.Fatal("BindsMismatchedArity rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "BindsMismatchedArity.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "BindsMismatchedArity.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(negativePath)
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
		file, err := scanner.ParseFile(positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func runCrossFileRule(rule *v2rules.Rule, files []*scanner.File) []scanner.Finding {
	idx := scanner.BuildIndex(files, 1)
	ctx := &v2rules.Context{
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

	if !rule.Needs.Has(v2rules.NeedsCrossFile) {
		t.Fatal("AnvilMergeComponentEmptyScope does not declare NeedsCrossFile")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "AnvilMergeComponentEmptyScope.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "AnvilMergeComponentEmptyScope.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(positivePath)
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
		file, err := scanner.ParseFile(negativePath)
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

	if !rule.Needs.Has(v2rules.NeedsCrossFile) {
		t.Fatal("DeadBindings does not declare NeedsCrossFile")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "DeadBindings.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "DeadBindings.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(positivePath)
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
		file, err := scanner.ParseFile(negativePath)
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

func TestHiltEntryPointOnNonInterface(t *testing.T) {
	rule := buildRuleIndex()["HiltEntryPointOnNonInterface"]
	if rule == nil {
		t.Fatal("HiltEntryPointOnNonInterface rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "di-hygiene", "HiltEntryPointOnNonInterface.kt")
	negativePath := filepath.Join(root, "negative", "di-hygiene", "HiltEntryPointOnNonInterface.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(negativePath)
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
		file, err := scanner.ParseFile(path)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", path, err)
		}
		parsed = append(parsed, file)
	}
	return parsed
}
