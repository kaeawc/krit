package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func benchmarkRuleByName(b *testing.B, ruleName string, code string) {
	b.Helper()
	dir := b.TempDir()
	path := filepath.Join(dir, "test.kt")
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		b.Fatal(err)
	}
	file, err := scanner.ParseFile(path)
	if err != nil {
		b.Fatalf("ParseFile(%s): %v", path, err)
	}
	var rule *v2rules.Rule
	for _, r := range v2rules.Registry {
		if r.ID == ruleName {
			rule = r
			break
		}
	}
	if rule == nil {
		b.Fatalf("rule %q not found", ruleName)
	}
	d := rules.NewDispatcherV2([]*v2rules.Rule{rule})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}

// --- Deprecation ---

func TestDeprecation_Positive(t *testing.T) {
	findings := runRuleByName(t, "Deprecation", `package test

@Deprecated("Use NewClass instead")
class OldClass

fun caller() {
    val x: OldClass = OldClass()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for using deprecated class")
	}
}

func TestDeprecation_Negative(t *testing.T) {
	findings := runRuleByName(t, "Deprecation", `
package test
fun currentMethod() {}

fun caller() {
    currentMethod()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- HasPlatformType ---

func TestHasPlatformType_Positive(t *testing.T) {
	findings := runRuleByName(t, "HasPlatformType", `
package test
fun getData() = JavaClass.getData()
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for public function without explicit return type")
	}
}

func TestHasPlatformType_Negative(t *testing.T) {
	findings := runRuleByName(t, "HasPlatformType", `
package test
fun getData(): String = JavaClass.getData()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestHasPlatformType_NegativeKotlinExpression(t *testing.T) {
	// Pure Kotlin expressions should not be flagged without resolver confirmation
	cases := []string{
		`package test
fun fromCode(code: Int) = entries.first { it.code == code }`,
		`package test
fun ensureDecryptionsDrained() = withTimeoutOrNull(5000) { drain() }`,
		`package test
fun getCurrentAvatar() = getState().currentAvatar`,
		`package test
fun count() = items.size`,
		`package test
fun name() = listOf("a", "b").first()`,
	}
	for i, src := range cases {
		findings := runRuleByName(t, "HasPlatformType", src)
		if len(findings) != 0 {
			t.Errorf("case %d: expected no findings for pure Kotlin expression, got %d", i, len(findings))
		}
	}
}

// --- IgnoredReturnValue ---

func TestIgnoredReturnValue_Positive(t *testing.T) {
	findings := runRuleByName(t, "IgnoredReturnValue", `
package test
fun main() {
    val list = listOf(1, 2, 3)
    list.map { it * 2 }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for ignored return value of map")
	}
}

func TestIgnoredReturnValue_Negative(t *testing.T) {
	findings := runRuleByName(t, "IgnoredReturnValue", `
package test
fun main() {
    val list = listOf(1, 2, 3)
    val doubled = list.map { it * 2 }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestIgnoredReturnValue_NoResolverSkipsSideEffectFunctionalNames(t *testing.T) {
	findings := runRuleByName(t, "IgnoredReturnValue", `
package test
class EffectSink {
    fun map(block: (Int) -> Unit): Unit {
        block(1)
    }

    fun filter(predicate: (Int) -> Boolean): Unit {
        predicate(1)
    }
}

fun f(sink: EffectSink) {
    sink.map { println(it) }
    sink.filter { it > 0 }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for side-effect methods, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_NoResolverKeepsFunctionalPipelineFallback(t *testing.T) {
	findings := runRuleByName(t, "IgnoredReturnValue", `
package test
fun sequence(): Sequence<Int> = sequenceOf(1, 2, 3)

fun f(items: List<Int>) {
    items.map { it + 1 }
    sequence().filter { it > 1 }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected two findings for discarded functional pipelines, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleReturnTypes(t *testing.T) {
	cases := []struct {
		name     string
		callText string
		rt       *typeinfer.ResolvedType
	}{
		{"sequence", "sequence().map { it + 1 }", &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}},
		{"flow", "flow().map { it + 1 }", &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}},
		{"stateFlow", "stateFlow().map { it + 1 }", &typeinfer.ResolvedType{Name: "StateFlow", FQN: "kotlinx.coroutines.flow.StateFlow", Kind: typeinfer.TypeClass}},
		{"stream", "stream().map { it + 1 }", &typeinfer.ResolvedType{Name: "Stream", FQN: "java.util.stream.Stream", Kind: typeinfer.TypeClass}},
		{"function", "factory()", &typeinfer.ResolvedType{Name: "Function1", FQN: "kotlin.Function1", Kind: typeinfer.TypeFunction}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := fmt.Sprintf(`package test
fun f() {
    %s
}
`, tc.callText)
			findings := runIgnoredReturnValueWithOracle(t, code, tc.callText, tc.rt, nil)
			if len(findings) != 1 {
				t.Fatalf("expected one finding for %s, got %d", tc.callText, len(findings))
			}
		})
	}
}

func TestIgnoredReturnValue_OracleSkipsUnitNothingAndUsedExpressions(t *testing.T) {
	cases := []struct {
		name     string
		code     string
		callText string
		rt       *typeinfer.ResolvedType
	}{
		{"unit", `package test
fun log(): Unit = Unit
fun f() { log() }`, "log()", &typeinfer.ResolvedType{Name: "Unit", FQN: "kotlin.Unit", Kind: typeinfer.TypeUnit}},
		{"nothing", `package test
fun fail(): Nothing = error("x")
fun f() { fail() }`, "fail()", &typeinfer.ResolvedType{Name: "Nothing", FQN: "kotlin.Nothing", Kind: typeinfer.TypeNothing}},
		{"unitFunctionalName", `package test
class Logger { fun map(block: () -> Unit): Unit = Unit }
fun f(logger: Logger) { logger.map { } }`, "logger.map { }", &typeinfer.ResolvedType{Name: "Unit", FQN: "kotlin.Unit", Kind: typeinfer.TypeUnit}},
		{"nonMustUseFunctionalName", `package test
fun f(list: List<Int>) { list.map { it + 1 } }`, "list.map { it + 1 }", &typeinfer.ResolvedType{Name: "List", FQN: "kotlin.collections.List", Kind: typeinfer.TypeClass}},
		{"assignment", `package test
fun sequence(): Sequence<Int> = sequenceOf(1)
fun f() { val xs = sequence() }`, "sequence()", &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}},
		{"return", `package test
fun sequence(): Sequence<Int> = sequenceOf(1)
fun f(): Sequence<Int> { return sequence() }`, "sequence()", &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}},
		{"lambdaResult", `package test
fun sequence(): Sequence<Int> = sequenceOf(1)
fun f(): Sequence<Int> = run { sequence() }`, "sequence()", &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runIgnoredReturnValueWithOracle(t, tc.code, tc.callText, tc.rt, nil)
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestIgnoredReturnValue_OracleAnnotations(t *testing.T) {
	checkReturn := []string{"com.google.errorprone.annotations.CheckReturnValue"}
	checkResult := []string{"androidx.annotation.CheckResult"}
	canIgnore := []string{"com.google.errorprone.annotations.CheckReturnValue", "com.google.errorprone.annotations.CanIgnoreReturnValue"}

	if findings := runIgnoredReturnValueWithOracle(t, `package test
fun buildToken(): String = "token"
fun f() { buildToken() }`, "buildToken()", &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypePrimitive}, checkReturn); len(findings) != 1 {
		t.Fatalf("expected @CheckReturnValue finding, got %d", len(findings))
	}
	if findings := runIgnoredReturnValueWithOracle(t, `package test
fun buildToken(): String = "token"
fun f() { buildToken() }`, "buildToken()", &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypePrimitive}, checkResult); len(findings) != 1 {
		t.Fatalf("expected @CheckResult finding, got %d", len(findings))
	}
	if findings := runIgnoredReturnValueWithOracle(t, `package test
fun maybeBuild(): String = "token"
fun f() { maybeBuild() }`, "maybeBuild()", &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypePrimitive}, canIgnore); len(findings) != 0 {
		t.Fatalf("expected @CanIgnoreReturnValue suppression, got %d", len(findings))
	}

	t.Run("check return on containing declaration", func(t *testing.T) {
		findings := runIgnoredReturnValueWithOracleEvidence(t, `
package test
class TokenBuilder {
    fun build(): String = "token"
}
fun f(builder: TokenBuilder) {
    builder.build()
}
`, oracleCallEvidence{
			CallText:   "builder.build()",
			CallTarget: "test.TokenBuilder.build",
			ContainerAnnotations: map[string][]string{
				"test.TokenBuilder": {"com.google.errorprone.annotations.CheckReturnValue"},
			},
		})
		if len(findings) != 1 {
			t.Fatalf("expected one finding, got %d: %#v", len(findings), findings)
		}
	})

	t.Run("can ignore on containing declaration", func(t *testing.T) {
		findings := runIgnoredReturnValueWithOracleEvidence(t, `
package test
class TokenBuilder {
    fun build(): String = "token"
}
fun f(builder: TokenBuilder) {
    builder.build()
}
`, oracleCallEvidence{
			CallText:    "builder.build()",
			CallTarget:  "test.TokenBuilder.build",
			Annotations: []string{"com.google.errorprone.annotations.CheckReturnValue"},
			ContainerAnnotations: map[string][]string{
				"test.TokenBuilder": {"com.google.errorprone.annotations.CanIgnoreReturnValue"},
			},
		})
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %d: %#v", len(findings), findings)
		}
	})
}

type oracleCallEvidence struct {
	CallText             string
	CallTarget           string
	ReturnType           *typeinfer.ResolvedType
	Annotations          []string
	ContainerAnnotations map[string][]string
}

func runIgnoredReturnValueWithOracle(t *testing.T, code, callText string, rt *typeinfer.ResolvedType, annotations []string) []scanner.Finding {
	return runIgnoredReturnValueWithOracleEvidence(t, code, oracleCallEvidence{
		CallText:    callText,
		ReturnType:  rt,
		Annotations: annotations,
	})
}

func runIgnoredReturnValueWithOracleEvidence(t *testing.T, code string, ev oracleCallEvidence) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{}
	fake.CallTargets[file.Path] = map[string]string{}
	fake.CallTargetAnnotations[file.Path] = map[string][]string{}
	for target, annotations := range ev.ContainerAnnotations {
		fake.Annotations[target] = annotations
	}
	var matched bool
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		text := strings.TrimSpace(file.FlatNodeText(idx))
		if !strings.Contains(text, ev.CallText) {
			return
		}
		key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
		if ev.ReturnType != nil {
			fake.Expressions[file.Path][key] = ev.ReturnType
		}
		if ev.CallTarget != "" {
			fake.CallTargets[file.Path][key] = ev.CallTarget
		}
		if len(ev.Annotations) > 0 {
			fake.CallTargetAnnotations[file.Path][key] = ev.Annotations
		}
		matched = true
	})
	if !matched {
		t.Fatalf("call %q not found", ev.CallText)
	}
	resolver := oracle.NewCompositeResolver(fake, typeinfer.NewFakeResolver())
	for _, r := range v2rules.Registry {
		if r.ID == "IgnoredReturnValue" {
			d := rules.NewDispatcherV2([]*v2rules.Rule{r}, resolver)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("IgnoredReturnValue rule not found")
	return nil
}

// --- ImplicitDefaultLocale ---

func TestImplicitDefaultLocale_Positive(t *testing.T) {
	findings := runRuleByName(t, "ImplicitDefaultLocale", `
package test
fun main() {
    val s = "Hello"
    s.toLowerCase()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for toLowerCase() without Locale")
	}
}

func TestImplicitDefaultLocale_Negative(t *testing.T) {
	findings := runRuleByName(t, "ImplicitDefaultLocale", `
package test
import java.util.Locale
fun main() {
    val s = "Hello"
    s.toLowerCase(Locale.ROOT)
    // Kotlin 1.5+ lowercase() is locale-invariant (Locale.ROOT) by design.
    s.lowercase()
    s.uppercase()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestImplicitDefaultLocale_FormatSpecifierPolicy(t *testing.T) {
	tests := []struct {
		name         string
		call         string
		wantFindings int
	}{
		{name: "integer digits", call: `String.format("%d", count)`, wantFindings: 1},
		{name: "grouped integer", call: `String.format("%,d", count)`, wantFindings: 1},
		{name: "floating point", call: `String.format("%.2f", value)`, wantFindings: 1},
		{name: "date time", call: `String.format("%tF", now)`, wantFindings: 1},
		{name: "string", call: `String.format("%s", name)`, wantFindings: 0},
		{name: "escaped percent", call: `String.format("progress %% done")`, wantFindings: 0},
		{name: "escaped percent before d", call: `String.format("%%d")`, wantFindings: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := runRuleByName(t, "ImplicitDefaultLocale", `
package test
fun main(now: Long, value: Double, count: Int, name: String) {
    `+tt.call+`
}
`)
			if len(findings) != tt.wantFindings {
				t.Fatalf("expected %d findings for %s, got %d", tt.wantFindings, tt.call, len(findings))
			}
		})
	}
}

func TestImplicitDefaultLocale_FormatPositiveSpecifiers(t *testing.T) {
	findings := runRuleByName(t, "ImplicitDefaultLocale", `
package test
fun main(now: Long, value: Double, count: Int) {
    String.format("Audio: %d Kbit/s", count)
    String.format("%,d", count)
    "%.2f".format(value)
    "%tF".format(now)
    "Timestamp: %d".format(now)
}
`)
	if len(findings) != 5 {
		t.Fatalf("expected 5 findings for locale-sensitive format specifiers, got %d", len(findings))
	}
}

func TestImplicitDefaultLocale_FormatNegativeSpecifiers(t *testing.T) {
	findings := runRuleByName(t, "ImplicitDefaultLocale", `
package test
import java.util.Locale
fun main(value: Double, count: Int, name: String) {
    String.format(Locale.US, "Audio: %d Kbit/s", count)
    "%.2f".format(Locale.US, value)
    String.format("%s", name)
    "progress %% done".format()
    "%%d".format()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for explicit Locale or locale-insensitive specifiers, got %d", len(findings))
	}
}

func TestImplicitDefaultLocale_OracleConfirmsStringFormatTargets(t *testing.T) {
	findings := runRuleByNameWithCallTarget(t, "ImplicitDefaultLocale", `
package test
fun main(count: Int) {
    String.format("Audio: %d Kbit/s", count)
}
`, `String.format("Audio: %d Kbit/s", count)`, "java.lang.String.format")
	if len(findings) != 1 {
		t.Fatalf("expected finding for resolved java.lang.String.format, got %d", len(findings))
	}

	findings = runRuleByNameWithCallTarget(t, "ImplicitDefaultLocale", `
package test
fun main(value: Double) {
    "%.2f".format(value)
}
`, `"%.2f".format(value)`, "kotlin.text.format")
	if len(findings) != 1 {
		t.Fatalf("expected finding for resolved kotlin.text.format, got %d", len(findings))
	}
}

func TestImplicitDefaultLocale_EmptyOracleFallsBackToAst(t *testing.T) {
	file := parseInline(t, `
package test
fun main(count: Int) {
    "%d".format(count)
}
`)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	composite := oracle.NewCompositeResolver(oracle.NewFakeOracle(), resolver)

	for _, r := range v2rules.Registry {
		if r.ID == "ImplicitDefaultLocale" {
			dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{r}, composite)
			cols := dispatcher.Run(file)
			findings := cols.Findings()
			if len(findings) != 1 {
				t.Fatalf("expected AST fallback finding when oracle has no call target, got %d", len(findings))
			}
			return
		}
	}
	t.Fatal("rule \"ImplicitDefaultLocale\" not found in registry")
}

func TestImplicitDefaultLocale_LexicalOracleTargetFallsBackToAst(t *testing.T) {
	findings := runRuleByNameWithCallTarget(t, "ImplicitDefaultLocale", `
package test
fun main(count: Int) {
    "%d".format(count)
}
`, `"%d".format(count)`, "format")
	if len(findings) != 1 {
		t.Fatalf("expected AST fallback finding when oracle target is lexical fallback, got %d", len(findings))
	}
}

func TestImplicitDefaultLocale_OracleSuppressesNonStringFormatTargets(t *testing.T) {
	findings := runRuleByNameWithCallTarget(t, "ImplicitDefaultLocale", `
package test
fun kotlin.String.format(value: Double): kotlin.String = this
fun main(value: Double) {
    "%.2f".format(value)
}
`, `"%.2f".format(value)`, "test.format")
	if len(findings) != 0 {
		t.Fatalf("expected no findings for resolved user-defined format extension, got %d", len(findings))
	}
}

// --- LocaleDefaultForCurrency ---

func TestLocaleDefaultForCurrency_Positive(t *testing.T) {
	findings := runRuleByName(t, "LocaleDefaultForCurrency", `
package test

import java.util.Currency
import java.util.Locale

class PriceFormatter {
    private val currency = Currency.getInstance(Locale.getDefault())
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for currency derived from Locale.getDefault() in a price-related class")
	}
}

func TestLocaleDefaultForCurrency_Negative(t *testing.T) {
	findings := runRuleByName(t, "LocaleDefaultForCurrency", `
package test

import java.util.Currency
import java.util.Locale

class DisplayFormatter {
    private val currency = Currency.getInstance(Locale.getDefault())
}

class PriceFormatter(private val currencyCode: String) {
    private val currency = Currency.getInstance(currencyCode)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func BenchmarkIgnoredReturnValue_DiscardedMap(b *testing.B) {
	benchmarkRuleByName(b, "IgnoredReturnValue", `
package test
fun main() {
    val list = listOf(1, 2, 3)
    list.map { it * 2 }
}
`)
}

func BenchmarkImplicitDefaultLocale_FormatWithExplicitLocale(b *testing.B) {
	benchmarkRuleByName(b, "ImplicitDefaultLocale", `
package test
import java.util.Locale
fun main() {
    val s = "Hello"
    String.format(Locale.ROOT, "%s", s)
}
`)
}
