// Package ruletest is the canonical entry point for unit-testing rules.
//
// Detekt's testing API splits along the type-resolution axis: lint() for
// rules that only need PSI and lintWithContext() for rules that need a
// resolved BindingContext. The Krit equivalent splits along capability
// tiers — each helper here exposes exactly one tier, and panics with a
// clear message when the rule under test declares more than the tier
// can provide. That makes capability declarations executable: a rule
// that quietly grew a NeedsResolver dependency will fail loudly the
// first time a syntax-only test runs against it.
//
// Tiers, narrowest first:
//
//   - LintSource / LintSourceJava — AST/import-only. The rule's Needs
//     bitfield must be zero (or contain only NeedsLinePass /
//     NeedsAggregate / NeedsConcurrent, none of which require external
//     facts).
//   - LintWithResolver / LintWithResolverJava — adds a source-level
//     typeinfer.TypeResolver indexed over the file. Allowed for rules
//     declaring NeedsResolver. NeedsOracle is rejected; use the oracle
//     helpers instead.
//   - LintWithFakeOracle / LintWithFakeOracleJava — adds a CompositeResolver
//     that wraps the supplied FakeOracle in front of the source-level
//     resolver. Allowed for rules declaring NeedsResolver and / or
//     NeedsOracle.
//
// Project-scope capabilities (NeedsCrossFile, NeedsModuleIndex,
// NeedsParsedFiles, NeedsManifest, NeedsResources, NeedsGradle) are
// rejected by every helper — those rules need a real pipeline phase
// beyond what a single-file unit test can provide. Tests for them
// should use the integration harness instead.
package ruletest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// projectScopeCaps are the capability bits that require a real pipeline
// phase. None of the helpers here can provide them.
const projectScopeCaps = api.NeedsCrossFile | api.NeedsModuleIndex |
	api.NeedsParsedFiles | api.NeedsManifest | api.NeedsResources | api.NeedsGradle

// LintSource parses src as Kotlin and runs ruleID against it without a
// resolver, oracle, or any cross-file context. Use for rules whose
// Needs bitfield is zero or only contains line-pass / aggregate /
// concurrent aspects.
//
// Fails the test (via t.Fatalf) if ruleID is not registered, if the
// rule declares any capability the tier cannot satisfy, or if parsing
// fails.
func LintSource(t *testing.T, ruleID, src string) []scanner.Finding {
	t.Helper()
	rule := requireRule(t, ruleID)
	enforceTier(t, rule, "LintSource", 0)
	file := writeAndParseKotlin(t, src)
	return runDispatcher(t, rule, file, nil)
}

// LintSourceJava is the Java counterpart to LintSource. Same tier
// constraints apply.
func LintSourceJava(t *testing.T, ruleID, src string) []scanner.Finding {
	t.Helper()
	rule := requireRule(t, ruleID)
	enforceTier(t, rule, "LintSourceJava", 0)
	file := writeAndParseJava(t, src)
	return runDispatcher(t, rule, file, nil)
}

// LintWithResolver parses src as Kotlin and runs ruleID with a
// source-level TypeResolver indexed over the file. Use for rules
// declaring NeedsResolver but not NeedsOracle.
func LintWithResolver(t *testing.T, ruleID, src string) []scanner.Finding {
	t.Helper()
	rule := requireRule(t, ruleID)
	enforceTier(t, rule, "LintWithResolver", api.NeedsResolver)
	file := writeAndParseKotlin(t, src)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	return runDispatcher(t, rule, file, resolver)
}

// LintWithResolverJava is the Java counterpart to LintWithResolver.
func LintWithResolverJava(t *testing.T, ruleID, src string) []scanner.Finding {
	t.Helper()
	rule := requireRule(t, ruleID)
	enforceTier(t, rule, "LintWithResolverJava", api.NeedsResolver)
	file := writeAndParseJava(t, src)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	return runDispatcher(t, rule, file, resolver)
}

// LintWithFakeOracle parses src as Kotlin and runs ruleID with a
// CompositeResolver that consults fake before falling back to the
// source-level TypeResolver. Use for rules declaring NeedsOracle (and
// optionally NeedsResolver) when a unit test wants to seed specific
// expression types or call-target stubs without standing up the real
// oracle daemon.
//
// Pass nil for fake to wire only the source-level resolver — useful
// when the rule's tested code path doesn't actually consult the
// oracle for the snippet under test but the rule's declared Needs
// includes NeedsOracle.
func LintWithFakeOracle(t *testing.T, ruleID, src string, fake *oracle.FakeOracle) []scanner.Finding {
	t.Helper()
	rule := requireRule(t, ruleID)
	enforceTier(t, rule, "LintWithFakeOracle", api.NeedsResolver|api.NeedsOracle)
	file := writeAndParseKotlin(t, src)
	resolver := makeOracleResolver(file, fake)
	return runDispatcher(t, rule, file, resolver)
}

// LintWithFakeOracleJava is the Java counterpart to LintWithFakeOracle.
func LintWithFakeOracleJava(t *testing.T, ruleID, src string, fake *oracle.FakeOracle) []scanner.Finding {
	t.Helper()
	rule := requireRule(t, ruleID)
	enforceTier(t, rule, "LintWithFakeOracleJava", api.NeedsResolver|api.NeedsOracle)
	file := writeAndParseJava(t, src)
	resolver := makeOracleResolver(file, fake)
	return runDispatcher(t, rule, file, resolver)
}

// ErrRuleNotRegistered is returned by lookupRule when the requested
// rule ID is not present in api.Registry. Exposed so tests for the
// validation layer can match the error type without parsing strings.
var ErrRuleNotRegistered = errors.New("rule not registered")

func requireRule(t *testing.T, ruleID string) *api.Rule {
	t.Helper()
	rule, err := lookupRule(ruleID)
	if err != nil {
		t.Fatalf("ruletest: %v", err)
	}
	return rule
}

// lookupRule returns the registered rule with the given ID, or
// ErrRuleNotRegistered when no such rule exists.
func lookupRule(ruleID string) (*api.Rule, error) {
	for _, r := range api.Registry {
		if r != nil && r.ID == ruleID {
			return r, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrRuleNotRegistered, ruleID)
}

func enforceTier(t *testing.T, rule *api.Rule, helper string, allowed api.Capabilities) {
	t.Helper()
	if err := validateTier(rule, helper, allowed); err != nil {
		t.Fatal(err)
	}
}

// validateTier returns nil when rule's declared Needs are compatible
// with the helper's allowed-capability set, or a descriptive error
// otherwise.
//
// allowed is the union of capability bits the tier supplies in addition
// to the always-allowed scope-shape bits (NeedsLinePass / NeedsAggregate
// / NeedsConcurrent), which never demand external context.
//
// This function is package-private but tested directly by the ruletest
// test suite — calling t.Fatalf inside a sub-test propagates failure to
// the parent, so the validation layer needs to be testable without
// going through *testing.T.
func validateTier(rule *api.Rule, helper string, allowed api.Capabilities) error {
	if rule.Needs&projectScopeCaps != 0 {
		return fmt.Errorf("ruletest.%s: rule %q declares project-scope capability %v; use the integration harness instead",
			helper, rule.ID, rule.Needs&projectScopeCaps)
	}
	tierAllowed := allowed | api.NeedsLinePass | api.NeedsAggregate | api.NeedsConcurrent
	if extra := rule.Needs &^ tierAllowed; extra != 0 {
		return fmt.Errorf("ruletest.%s: rule %q declares capability %v which this tier cannot provide; use a higher-tier helper",
			helper, rule.ID, extra)
	}
	return nil
}

func writeAndParseKotlin(t *testing.T, src string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("ruletest: write Kotlin temp file: %v", err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatalf("ruletest: parse Kotlin: %v", err)
	}
	return file
}

func writeAndParseJava(t *testing.T, src string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Test.java")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("ruletest: write Java temp file: %v", err)
	}
	file, err := scanner.ParseJavaFile(context.Background(), path)
	if err != nil {
		t.Fatalf("ruletest: parse Java: %v", err)
	}
	return file
}

func makeOracleResolver(file *scanner.File, fake *oracle.FakeOracle) typeinfer.TypeResolver {
	source := typeinfer.NewResolver()
	source.IndexFilesParallel([]*scanner.File{file}, 1)
	if fake == nil {
		return source
	}
	return oracle.NewCompositeResolver(fake, source)
}

func runDispatcher(t *testing.T, rule *api.Rule, file *scanner.File, resolver typeinfer.TypeResolver) []scanner.Finding {
	t.Helper()
	var d *rules.Dispatcher
	if resolver != nil {
		d = rules.NewDispatcher([]*api.Rule{rule}, resolver)
	} else {
		d = rules.NewDispatcher([]*api.Rule{rule}, nil)
	}
	cols := d.Run(file)
	return cols.Findings()
}
