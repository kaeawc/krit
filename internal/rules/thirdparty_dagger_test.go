package rules_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// DaggerIssuesDetector's BindsWrongParameterCount and RedundantBinds are Lint aliases of existing krit rules, so
// suppressions and lint.xml entries written for the Lint IDs apply to them.
func TestDaggerLintIDsAliasExistingRules(t *testing.T) {
	for rule, lintID := range map[string]string{
		"BindsMismatchedArity":        "BindsWrongParameterCount",
		"BindsReturnTypeMatchesParam": "RedundantBinds",
	} {
		var r *api.Rule
		for _, candidate := range api.Registry {
			if candidate.ID == rule {
				r = candidate
			}
		}
		if r == nil {
			t.Fatalf("rule %s not registered", rule)
		}
		if aliases := rules.LintSuppressionAliases(r); !slices.Contains(aliases, lintID) {
			t.Errorf("%s Lint aliases = %v, want %s", rule, aliases, lintID)
		}
	}

	const src = `package test

import android.annotation.SuppressLint
import dagger.Binds
import dagger.Module

interface Foo
class FooImpl : Foo

@Module
interface FooModule {
  %s
  @Binds fun bind(a: FooImpl, b: FooImpl): Foo
}
`
	if got := runRuleByName(t, "BindsMismatchedArity", strings.Replace(src, "%s", "", 1)); len(got) != 1 {
		t.Fatalf("expected one BindsMismatchedArity finding without the annotation, got %d", len(got))
	}
	suppressed := strings.Replace(src, "%s", `@SuppressLint("BindsWrongParameterCount")`, 1)
	if got := runRuleByName(t, "BindsMismatchedArity", suppressed); len(got) != 0 {
		t.Errorf(`@SuppressLint("BindsWrongParameterCount") must suppress BindsMismatchedArity, got %d findings`, len(got))
	}
}

// A wrong parameter count is reported as BindsWrongParameterCount, which ends the detector's checks, so the later
// return-type and receiver checks stay silent even though the function also has those problems.
func TestDaggerWrongParameterCountHidesLaterChecks(t *testing.T) {
	const src = `package test

import dagger.Binds
import dagger.Module

@Module
interface FooModule {
  @Binds fun none()
  @Binds fun String.two(other: String): CharSequence
}
`
	for _, rule := range []string{"BindingReturnType", "BindingReceiverParameter"} {
		if got := runRuleByName(t, rule, src); len(got) != 0 {
			t.Errorf("%s: expected no findings after a wrong parameter count, got %v", rule, got)
		}
	}
}
