package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
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
