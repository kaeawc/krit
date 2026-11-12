package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// --- UnsafeCast ---

func TestUnsafeCast_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
fun process() {
    val str = 1 as String
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for impossible cast 'Int as String', got none")
	}
}

func runRuleByNameWithResolver(t *testing.T, ruleName string, code string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	for _, r := range api.Registry {
		if r.ID == ruleName {
			d := rules.NewDispatcher([]*api.Rule{r}, resolver)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

func runRuleByNameWithCallTarget(t *testing.T, ruleName string, code string, callText string, callTarget string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if strings.TrimSpace(file.FlatNodeText(idx)) == callText {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = callTarget
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == ruleName {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

func runRuleByNameWithOracleDiagnostic(t *testing.T, ruleName string, code string, castText string, factoryName string) []scanner.Finding {
	return runRuleByNameWithOracleDiagnosticForNode(t, ruleName, code, "as_expression", castText, factoryName)
}

func runRuleByNameWithOracleDiagnosticForNode(t *testing.T, ruleName string, code string, nodeType string, nodeText string, factoryName string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	file.FlatWalkNodes(0, nodeType, func(idx uint32) {
		if strings.TrimSpace(file.FlatNodeText(idx)) == nodeText {
			fake.Diagnostics[file.Path] = []oracle.Diagnostic{{
				FactoryName: factoryName,
				Severity:    "WARNING",
				Message:     "This cast can never succeed",
				Line:        file.FlatRow(idx) + 1,
				Col:         file.FlatCol(idx) + 1,
			}}
		}
	})
	if len(fake.Diagnostics[file.Path]) == 0 {
		t.Fatalf("%s node %q not found", nodeType, nodeText)
	}
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == ruleName {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

func runRuleByNameWithEmptyOracle(t *testing.T, ruleName string, code string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	composite := oracle.NewCompositeResolver(oracle.NewFakeOracle(), resolver)
	for _, r := range api.Registry {
		if r.ID == ruleName {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

// runRuleByNameWithAnalyzedOracle runs the rule against a compiler that analyzed the file and reported nothing.
func runRuleByNameWithAnalyzedOracle(t *testing.T, ruleName string, code string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.AnalyzedFiles[file.Path] = true
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == ruleName {
			cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

// The shadowing local `n` is a String, but the name-based resolver types it as the Int property of the same name.
const unsafeCastShadowedLocal = `
package test
class Holder(val n: Int) {
    fun text(): String {
        val s = listOf(1).map { n -> "x$n" }.first()
        return run { val n = "text"; n as String }
    }
}
`

func TestUnsafeCast_CompilerAnalyzedFileWithoutDiagnosticIsClean(t *testing.T) {
	if findings := runRuleByNameWithAnalyzedOracle(t, "UnsafeCast", unsafeCastShadowedLocal); len(findings) != 0 {
		t.Fatalf("the compiler analyzed the file and reported no CAST_NEVER_SUCCEEDS, got %d findings: %+v", len(findings), findings)
	}
}

func TestUnsafeCast_SourceFallbackRunsWithoutCompilerFacts(t *testing.T) {
	if findings := runRuleByNameWithEmptyOracle(t, "UnsafeCast", unsafeCastShadowedLocal); len(findings) != 1 {
		t.Fatalf("without compiler facts the source fallback must still run, got %d findings: %+v", len(findings), findings)
	}
}

func TestUnsafeCast_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
fun process(obj: Any) {
    val str = obj as? String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe cast 'as?', got %d", len(findings))
	}
}

func TestUnsafeCast_PositiveSafeCastNeverSucceeds(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
fun process() {
    val str = 1 as? String
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for impossible safe cast 'Int as? String', got none")
	}
	if findings[0].Fix != nil {
		t.Fatal("expected no autofix for a safe cast that is already using as?")
	}
}

func TestUnsafeCast_UsesOracleCastNeverSucceedsDiagnostic(t *testing.T) {
	findings := runRuleByNameWithOracleDiagnostic(t, "UnsafeCast", `
package test
fun process(obj: Any) {
    val str = obj as String
}
`, "obj as String", "CAST_NEVER_SUCCEEDS")
	if len(findings) == 0 {
		t.Fatal("expected oracle CAST_NEVER_SUCCEEDS diagnostic to emit UnsafeCast")
	}
}

func TestUnsafeCast_NestedDiagnosticNotClaimedByEnclosingCast(t *testing.T) {
	// The compiler emits CAST_NEVER_SUCCEEDS on the inner cast `obj as Foo`.
	// The enclosing cast `(obj as Foo) as Bar` contains that range, so a plain
	// overlap check would wrongly attribute the diagnostic (and its deleting
	// fix) to the outer cast too. Foo/Bar are open classes, so the local
	// never-succeeds fallback stays off and only the diagnostic path can fire.
	findings := runRuleByNameWithOracleDiagnosticForNode(t, "UnsafeCast", `
package test
open class Foo
open class Bar
fun process(obj: Any) {
    val x = (obj as Foo) as Bar
}
`, "as_expression", "obj as Foo", "CAST_NEVER_SUCCEEDS")
	if len(findings) != 1 {
		t.Fatalf("nested cast diagnostic must be claimed only by the inner cast, got %d findings: %+v", len(findings), findings)
	}
}

func TestUnsafeCast_UsesOracleCastNeverSucceedsDiagnosticForSafeCast(t *testing.T) {
	findings := runRuleByNameWithOracleDiagnostic(t, "UnsafeCast", `
package test
fun process(obj: Any) {
    val str = obj as? String
}
`, "obj as? String", "CAST_NEVER_SUCCEEDS")
	if len(findings) == 0 {
		t.Fatal("expected oracle CAST_NEVER_SUCCEEDS diagnostic to emit UnsafeCast for as?")
	}
	if findings[0].Fix != nil {
		t.Fatal("expected no autofix for oracle-reported safe cast")
	}
}

// --- UselessElvisOnNonNull ---

const uselessElvisOnNonNullSource = `
package test
fun process() {
    val name: String = "ready"
    val result = name ?: "fallback"
}
`

func TestUselessElvisOnNonNull_UsesOracleUselessElvisDiagnostic(t *testing.T) {
	findings := runRuleByNameWithOracleDiagnosticForNode(t, "UselessElvisOnNonNull", uselessElvisOnNonNullSource,
		"elvis_expression", `name ?: "fallback"`, "USELESS_ELVIS")
	if len(findings) != 1 {
		t.Fatalf("expected one projected finding, got %d", len(findings))
	}
	if findings[0].Confidence != api.ConfidenceVeryHigh {
		t.Fatalf("projected confidence = %v, want %v", findings[0].Confidence, api.ConfidenceVeryHigh)
	}
	if got, want := findings[0].Message, "Useless elvis (?:) on non-nullable 'name'. The fallback is dead code."; got != want {
		t.Fatalf("projected message = %q, want %q", got, want)
	}
}

func TestUselessElvisOnNonNull_UsesHeuristicWithoutOracle(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UselessElvisOnNonNull", uselessElvisOnNonNullSource)
	if len(findings) != 1 {
		t.Fatalf("expected one heuristic finding, got %d", len(findings))
	}
	if findings[0].Confidence != api.ConfidenceHigh {
		t.Fatalf("heuristic confidence = %v, want %v", findings[0].Confidence, api.ConfidenceHigh)
	}
	if got, want := findings[0].Message, "Useless elvis (?:) on non-nullable 'name'. The fallback is dead code."; got != want {
		t.Fatalf("heuristic message = %q, want %q", got, want)
	}
}

func TestUselessElvisOnNonNull_FallsBackWhenOracleHasNoDiagnostics(t *testing.T) {
	findings := runRuleByNameWithEmptyOracle(t, "UselessElvisOnNonNull", uselessElvisOnNonNullSource)
	if len(findings) != 1 {
		t.Fatalf("expected one fallback finding, got %d", len(findings))
	}
	if findings[0].Confidence != api.ConfidenceHigh {
		t.Fatalf("fallback confidence = %v, want %v", findings[0].Confidence, api.ConfidenceHigh)
	}
}

func TestUselessElvisOnNonNull_ProjectsForNonLocalIdentifier(t *testing.T) {
	// CONFIG is imported, not declared in-file, so uselessElvisOperand's
	// same-file-target heuristic refuses it and the heuristic path emits
	// nothing. The projection must still fire on the compiler's authoritative
	// USELESS_ELVIS verdict — otherwise a compiler-confirmed finding is lost.
	const src = `
package test
import com.example.CONFIG
fun process() {
    val result = CONFIG ?: "fallback"
}
`
	findings := runRuleByNameWithOracleDiagnosticForNode(t, "UselessElvisOnNonNull", src,
		"elvis_expression", `CONFIG ?: "fallback"`, "USELESS_ELVIS")
	if len(findings) != 1 {
		t.Fatalf("expected one projected finding for a non-local identifier, got %d", len(findings))
	}
	if findings[0].Confidence != api.ConfidenceVeryHigh {
		t.Fatalf("projected confidence = %v, want %v", findings[0].Confidence, api.ConfidenceVeryHigh)
	}
	if got, want := findings[0].Message, "Useless elvis (?:) on non-nullable 'CONFIG'. The fallback is dead code."; got != want {
		t.Fatalf("projected message = %q, want %q", got, want)
	}
}

func TestUnnecessaryNotNullOperator_ProjectsCompilerDiagnostic(t *testing.T) {
	// CONFIG is imported, so the name-based heuristic can't prove it non-null
	// and emits nothing; the projection must fire on the compiler's
	// UNNECESSARY_NOT_NULL_ASSERTION verdict.
	findings := runRuleByNameWithOracleDiagnosticForNode(t, "UnnecessaryNotNullOperator", `
package test
import com.example.CONFIG
fun f() {
    val x = CONFIG!!
}
`, "postfix_expression", "CONFIG!!", "UNNECESSARY_NOT_NULL_ASSERTION")
	if len(findings) != 1 {
		t.Fatalf("expected one projected finding, got %d", len(findings))
	}
	if findings[0].Confidence != api.ConfidenceVeryHigh {
		t.Fatalf("projected confidence = %v, want VeryHigh", findings[0].Confidence)
	}
}

func TestUnnecessaryNotNullOperator_SmartCastWording(t *testing.T) {
	message := func(code, nodeText string) string {
		findings := runRuleByNameWithOracleDiagnosticForNode(t, "UnnecessaryNotNullOperator", code, "postfix_expression", nodeText, "UNNECESSARY_NOT_NULL_ASSERTION")
		if len(findings) != 1 {
			t.Fatalf("expected one projected finding for %s, got %d", nodeText, len(findings))
		}
		return findings[0].Message
	}
	// A member access may be non-null only through an earlier assertion, so it must not be called non-nullable.
	member := message(`
package test
import com.example.Tag
fun f(tag: Tag) {
    check(tag.style!!.bold)
    check(tag.style!!.italic)
}
`, "tag.style!!")
	if strings.Contains(member, "non-nullable") || !strings.Contains(member, "'tag.style'") {
		t.Fatalf("member access message should not claim a non-nullable type: %q", member)
	}
	nullable := message(`
package test
fun f(name: String?) {
    if (name != null) {
        check(name!!.isEmpty())
    }
}
`, "name!!")
	if strings.Contains(nullable, "non-nullable") || !strings.Contains(nullable, "smart-cast") {
		t.Fatalf("nullable-declared receiver should be described as smart-cast: %q", nullable)
	}
	plain := message(`
package test
import com.example.CONFIG
fun f() {
    val x = CONFIG!!
}
`, "CONFIG!!")
	if !strings.Contains(plain, "non-nullable 'CONFIG'") {
		t.Fatalf("unknown simple receiver keeps the non-nullable wording: %q", plain)
	}
}

func TestUnnecessarySafeCall_ProjectsCompilerDiagnostic(t *testing.T) {
	findings := runRuleByNameWithOracleDiagnosticForNode(t, "UnnecessarySafeCall", `
package test
import com.example.CONFIG
fun f() {
    val x = CONFIG?.length
}
`, "navigation_expression", "CONFIG?.length", "UNNECESSARY_SAFE_CALL")
	if len(findings) != 1 {
		t.Fatalf("expected one projected finding, got %d", len(findings))
	}
	if findings[0].Confidence != api.ConfidenceVeryHigh {
		t.Fatalf("projected confidence = %v, want VeryHigh", findings[0].Confidence)
	}
}

func TestUnnecessaryNotNullCheck_ProjectsNonNullDirectionOnly(t *testing.T) {
	run := func(nodeText, message string) []scanner.Finding {
		file := parseInline(t, `
package test
import com.example.CONFIG
fun f(): Boolean { return CONFIG != null }
`)
		resolver := typeinfer.NewResolver()
		resolver.IndexFilesParallel([]*scanner.File{file}, 1)
		fake := oracle.NewFakeOracle()
		file.FlatWalkNodes(0, "equality_expression", func(idx uint32) {
			if strings.TrimSpace(file.FlatNodeText(idx)) == nodeText {
				fake.Diagnostics[file.Path] = []oracle.Diagnostic{{
					FactoryName: "SENSELESS_COMPARISON",
					Severity:    "WARNING",
					Message:     message,
					Line:        file.FlatRow(idx) + 1,
					Col:         file.FlatCol(idx) + 1,
				}}
			}
		})
		composite := oracle.NewCompositeResolver(fake, resolver)
		for _, r := range api.Registry {
			if r.ID == "UnnecessaryNotNullCheck" {
				cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
				return cols.Findings()
			}
		}
		t.Fatal("UnnecessaryNotNullCheck not found")
		return nil
	}
	// `CONFIG != null` proven always true → operand non-null → projects.
	if f := run("CONFIG != null", "Condition is always 'true'."); len(f) != 1 {
		t.Fatalf("non-null direction must project, got %d: %+v", len(f), f)
	}
	// `CONFIG != null` proven always false → operand always null → this rule
	// (about non-nullable operands) must NOT claim it.
	if f := run("CONFIG != null", "Condition is always 'false'."); len(f) != 0 {
		t.Fatalf("always-null direction must not project, got %d: %+v", len(f), f)
	}
}

func TestUnsafeCast_DoesNotFlagUnknownSubstringCallee(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
fun findViewByIdButNotReally(): Any = ""
fun process() {
    val str = findViewByIdButNotReally() as String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding without impossible-cast proof, got %d", len(findings))
	}
}

func TestUnsafeCast_SuppressesResolvedGetSystemServiceCast(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
import android.app.AlarmManager
import android.content.Context

fun process(context: Context) {
    val manager = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for Context.ALARM_SERVICE cast to AlarmManager, got %d", len(findings))
	}
}

func TestUnsafeCast_DoesNotTreatMismatchedGetSystemServiceAsNeverSucceeds(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
import android.content.Context
import android.os.PowerManager

fun process(context: Context) {
    val manager = context.getSystemService(Context.ALARM_SERVICE) as PowerManager
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no UnsafeCast for Context.ALARM_SERVICE cast without compiler proof, got %d", len(findings))
	}
}

func TestUnsafeCast_DoesNotFlagCommonAndroidFrameworkCasts(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test

const val NOTIFICATION_SERVICE = "notification"

open class Activity
class RestoreActivity : Activity()
open class Fragment {
    fun requireActivity(): Activity = Activity()
}
class NotificationManager
class Context {
    fun getSystemService(name: String): Any = NotificationManager()
}
open class ViewGroupLayoutParams
class FlexboxLayout {
    class LayoutParams : ViewGroupLayoutParams()
}
class View {
    val layoutParams: ViewGroupLayoutParams = FlexboxLayout.LayoutParams()
}

class Samples : Fragment() {
    fun restore() {
        val activity = requireActivity() as RestoreActivity
    }
    fun service(context: Context) {
        val notificationManager = context.getSystemService(NOTIFICATION_SERVICE) as NotificationManager
    }
    fun flex(child: View) {
        val params = child.layoutParams as FlexboxLayout.LayoutParams
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no UnsafeCast findings for common Android/framework casts, got %d", len(findings))
	}
}

func TestUnsafeCast_SuppressesUnqualifiedGetSystemServiceInContextOwner(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
import android.app.Service

class SyncService : Service() {
    fun process() {
        val manager = getSystemService(POWER_SERVICE) as android.os.PowerManager
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for POWER_SERVICE cast inside Service owner, got %d", len(findings))
	}
}

func TestUnsafeCast_FallsBackAfterLexicalOracleCallTarget(t *testing.T) {
	code := `
package test
import android.app.Service

class SyncService : Service() {
    fun process() {
        val manager = getSystemService(POWER_SERVICE) as android.os.PowerManager
    }
}
`
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if strings.Contains(file.FlatNodeText(idx), "getSystemService") {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = "getSystemService"
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)

	for _, r := range api.Registry {
		if r.ID != "UnsafeCast" {
			continue
		}
		cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
		findings := cols.Findings()
		if len(findings) != 0 {
			t.Fatalf("expected lexical oracle call target to fall back to resolver checks, got %d findings", len(findings))
		}
		return
	}
	t.Fatal("UnsafeCast rule not found")
}

func TestUnsafeCast_IgnoresMultiplatformTestRoots(t *testing.T) {
	for _, root := range []string{"commonJvmTest", "browserCommonTest", "jvmCommonTest"} {
		t.Run(root, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "src", root, "kotlin", "com", "example", "UnsafeCastTest.kt")
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			code := `
package test
fun process(record: Any) {
    val text = (record as String)
}
`
			if err := os.WriteFile(path, []byte(code), 0644); err != nil {
				t.Fatal(err)
			}
			file, err := scanner.ParseFile(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range api.Registry {
				if r.ID != "UnsafeCast" {
					continue
				}
				findingCols := rules.NewDispatcher([]*api.Rule{r}, nil).Run(file)
				if findingCols.Len() != 0 {
					t.Fatalf("expected no findings for %s source set, got %d", root, findingCols.Len())
				}
				return
			}
			t.Fatal("UnsafeCast rule not found in registry")
		})
	}
}

// --- UnsafeCallOnNullableType ---

func TestUnsafeCallOnNullableType_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun greet(name: String?) {
    val len = name!!.length
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for !! operator, got none")
	}
}

func TestUnsafeCallOnNullableType_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun greet(name: String?) {
    val len = name?.length
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe call ?., got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_MessageInsideRxMapperAvoidsSafeCall(t *testing.T) {
	const plain = "Calling !! on a nullable type will throw a NullPointerException at runtime in case the value is null. It should be avoided. Prefer requireNotNull(x) { \"reason\" } or checkNotNull(x) { \"reason\" }; use a safe call (?.) only when null is a valid case."
	rx := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
import io.reactivex.rxjava3.core.Observable
fun names(source: Observable<User>) = source.map { it.name!! }
fun loads(source: Observable<User>) = source.flatMap { val key = it.label!!; load(key) }
`)
	if len(rx) != 2 {
		t.Fatalf("both bangs stay flagged, got %d: %v", len(rx), rx)
	}
	for _, f := range rx {
		if f.Message == plain || !strings.Contains(f.Message, "requireNotNull") || !strings.Contains(f.Message, "checkNotNull") {
			t.Fatalf("an Rx mapper cannot return null, so suggest requireNotNull/checkNotNull: %q", f.Message)
		}
	}
	// Outside an Rx file, or inside another lambda, the plain advice stays.
	for name, src := range map[string]string{
		"no rx import": `
package test
fun names(source: List<User>) = source.map { it.name!! }
`,
		"non-mapper lambda": `
package test
import io.reactivex.rxjava3.core.Observable
fun names(source: Observable<User>) = source.subscribe { val shown = it.name!!; show(shown) }
`,
	} {
		f := runRuleByName(t, "UnsafeCallOnNullableType", src)
		if len(f) != 1 || f[0].Message != plain {
			t.Fatalf("%s: expected the plain message, got %v", name, f)
		}
	}
}

func TestUnsafeCallOnNullableType_MessagePrefersPreconditionsForPlatformAndArgumentShapes(t *testing.T) {
	cases := map[string]struct {
		src  string
		want string
	}{
		"super call": {`
package test
class Screen : BaseScreen() {
    override fun title(): String {
        return super.title()!!
    }
}
`, "platform-type"},
		"argument": {`
package test
fun show(name: String?) {
    render(name!!)
}
`, "requireNotNull"},
		"column read": {`
package test
fun read(reader: RowReader): ByteArray {
    return reader.getBlob(0)!!
}
`, "requireNotNull"},
	}
	for name, tc := range cases {
		f := runRuleByName(t, "UnsafeCallOnNullableType", tc.src)
		if len(f) != 1 {
			t.Fatalf("%s: expected one finding, got %v", name, f)
		}
		if !strings.Contains(f[0].Message, tc.want) || !strings.Contains(f[0].Message, "NotNull") || strings.Contains(f[0].Message, "Consider using safe calls") {
			t.Fatalf("%s: message should lead with a precondition and mention %q: %q", name, tc.want, f[0].Message)
		}
	}
	f := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
class Screen : BaseScreen() {
    override fun title(): String {
        return super.title()!!
    }
}
`)
	if !strings.Contains(f[0].Message, "checkNotNull") {
		t.Fatalf("platform call should suggest checkNotNull: %q", f[0].Message)
	}
}

func TestUnsafeCallOnNullableType_NegativeTestSupportSource(t *testing.T) {
	file := parseInline(t, `
package test

fun waitForFake(fake: Fake?) {
    fake!!.await()
}

class Fake {
    fun await() = Unit
}
`)
	file.Path = "/repo/camera/camera-testing/src/main/kotlin/androidx/camera/testing/Fake.kt"
	findings := runRuleByNameOnFile(t, "UnsafeCallOnNullableType", file)
	if len(findings) != 0 {
		t.Fatalf("expected test-support source to be ignored, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeAndroidTestSupportArtifactSource(t *testing.T) {
	file := parseInline(t, `
package test

class Activity {
    fun title(): String = ""
}

fun resumedActivity(activity: Activity?): String {
    return activity!!.title()
}
`)
	file.Path = "/repo/libraries/testing-ui/src/main/kotlin/com/example/TestRobot.kt"
	findings := runRuleByNameOnFile(t, "UnsafeCallOnNullableType", file)
	if len(findings) != 0 {
		t.Fatalf("expected Android test-support artifact source to be ignored, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeNonNullIfGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun greet(name: String?) {
    if (name != null) {
        val len = name!!.length
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for structurally guarded local parameter, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeEarlyReturnNullGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun greet(name: String?) {
    if (name == null) return
    val len = name!!.length
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for early-return null guard, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeThisQualifiedPropertyGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
class Greeter(private val name: String?) {
    fun greet() {
        if (this.name != null) {
            val len = name!!.length
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for this-qualified property guard, got %d", len(findings))
	}
}

// Regression: isGuardedNonNullFlat must locate the if-expression's condition by
// structural position rather than scanning children for the first node whose type
// is in an allowlist. The else-branch case exercises that structural ordering —
// condition, then-body, else-body — must be honored to credit the guard.
func TestUnsafeCallOnNullableType_NegativeElseBranchNullEqualsGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun greet(name: String?) {
    if (name == null) {
        return
    } else {
        val len = name!!.length
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for else-branch guarded receiver, got %d", len(findings))
	}
}

// Regression: nested if-expressions inside both branches put multiple
// condition-shaped children into the outer if's child list. The structural
// helper must still pick the outermost condition rather than the first node
// whose type matches an allowlist.
func TestUnsafeCallOnNullableType_NegativeNestedIfInBranchesGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun greet(name: String?, flag: Boolean) {
    if (name != null) {
        if (flag) {
            println(name!!.length)
        } else {
            println(name!!.uppercase())
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when outer guard proves non-null across nested branches, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_PositivePrefixOnlyGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
data class User(val name: String?)
fun greet(user: User?) {
    if (user != null) {
        val len = user.name!!.length
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when only receiver prefix is guarded")
	}
}

func TestUnsafeCallOnNullableType_PositiveFunctionCallGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun nextName(): String? = null
fun greet() {
    if (nextName() != null) {
        val len = nextName()!!.length
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for repeated function call null guard")
	}
}

func TestUnsafeCallOnNullableType_NegativeKspQualifiedName(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
import com.google.devtools.ksp.symbol.KSClassDeclaration

fun render(clazz: KSClassDeclaration) {
    val fqName = clazz.qualifiedName!!.asString()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for KSP qualifiedName unwrap, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeNamedKClassSimpleNameLiteral(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

class App

val tag = App::class.simpleName!!
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for named KClass simpleName literal, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_PositiveRuntimeKClassSimpleName(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

fun tag(value: Any): String {
    return value::class.simpleName!!
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for runtime KClass simpleName unwrap, got none")
	}
}

func TestUnsafeCallOnNullableType_PositiveQualifiedName(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
import kotlin.reflect.KClass

fun render(clazz: KClass<*>) {
    val fqName = clazz.qualifiedName!!
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for qualifiedName!! outside KSP, got none")
	}
}

func TestUnsafeCallOnNullableType_NegativeCreatorOrConstructorKsp(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import com.google.devtools.ksp.symbol.KSFunctionDeclaration

fun render(creatorOrConstructor: KSFunctionDeclaration?) {
    val name = creatorOrConstructor!!.simpleName.getShortName()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for creatorOrConstructor!! in KSP code, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_PositiveCreatorOrConstructor(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

fun render(creatorOrConstructor: String?) {
    val name = creatorOrConstructor!!
}
`)
	if len(findings) == 0 {
		t.Fatal("expected ordinary creatorOrConstructor!! to still be flagged, got none")
	}
}

func TestUnsafeCallOnNullableType_CompilerLookupPositive(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import org.jetbrains.kotlin.ir.util.referenceClass

fun process(pluginContext: Any, classId: Any) {
    val klass = pluginContext.referenceClass(classId)!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected compiler symbol lookup !! to be clean, got %d findings", len(findings))
	}
}

func TestUnsafeCallOnNullableType_CompilerLookupNegative(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import org.jetbrains.kotlin.ir.util.referenceClass

fun process(name: String?) {
    val len = name!!
}
`)
	if len(findings) == 0 {
		t.Fatal("expected ordinary !! inside compiler-importing file to still be flagged")
	}
}

func TestUnsafeCallOnNullableType_CompilerSymbolMetadataPositive(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import org.jetbrains.kotlin.backend.common.extensions.IrPluginContext

fun process(pluginContext: IrPluginContext, classId: Any) {
    val klass = pluginContext.referenceClass(classId)!!
    val companion = klass.companionObject()!!
    val fqName = companion.classId!!.asString()
    val creator = companion.creatorOrConstructor!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected compiler symbol metadata !! to be clean, got %d findings", len(findings))
	}
}

func TestUnsafeCallOnNullableType_CompilerPluginInvariantPositive(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import org.jetbrains.kotlin.backend.common.extensions.IrPluginContext
import org.jetbrains.kotlin.ir.declarations.IrProperty

fun process(pluginContext: IrPluginContext, property: IrProperty, requiredClassId: Any?) {
    val receiver = property.getter!!.dispatchReceiverParameter!!
    val field = property.backingField!!
    val callee = property.reference.callee!!
    val classId = requiredClassId!!
    val propertyType = generatedGraphMetadata!!.type
    val parentClass = parentClassOrNull!!
    val messageLocation = CompilerMessageLocationWithRange.create()!!
    val annotationScope = property.getAnnotation(pluginContext.classFqName)!!.scopeOrNull()!!
    val annotationName = property.getRequiredAnnotationString()!!
    val singleType = annotationScope.typeArguments.single()!!
    val dispatchReceiver = sizeSymbol!!
    val target = property.targetConstructor!!
    val generated = property.generatedGraphMetadata!!.typeKey
    consume(receiver, field, callee, classId, propertyType, parentClass, messageLocation, annotationScope, annotationName, singleType, dispatchReceiver, target, generated)
}

fun consume(vararg values: Any?) {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected compiler plugin invariant !! to be clean, got %d findings", len(findings))
	}
}

func TestUnsafeCallOnNullableType_CompilerSymbolMetadataNegative(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import org.jetbrains.kotlin.backend.common.extensions.IrPluginContext

fun process(pluginContext: IrPluginContext, name: String?) {
    val len = name!!
}
`)
	if len(findings) == 0 {
		t.Fatal("expected ordinary !! in compiler-importing file to still be flagged")
	}
}

func TestUnsafeCallOnNullableType_NegativePostFilterSmartCast(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

data class State(val inAppPaymentId: String?)

fun ids(states: List<State>): List<String> {
    return states
        .filter { it.inAppPaymentId != null }
        .map { it.inAppPaymentId!! }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for post-filter smart cast, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativePostFilterSmartCastNestedCallArg(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

data class State(val inAppPaymentId: String?)

class ViewModel(val state: List<State>)

fun consume(any: Any) {}

fun bind(viewModel: ViewModel) {
    consume(viewModel.state.filter { it.inAppPaymentId != null }.map { it.inAppPaymentId!! })
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nested-call post-filter smart cast, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeWhenBranchNullGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

data class ErrorPayload(val message: String)
data class Response(val onError: ErrorPayload?)

fun message(response: Response): String {
    return when {
        response.onError != null -> response.onError!!.message
        else -> ""
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for when-branch null guard, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeWhenBranchLongNavigationNullGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

class SummaryTemplate {
    fun toUiState(progress: Double, templateCommon: String): String = ""
}

fun screen(): String {
    val progress: Double? = 1.0
    val generalTemplate = ""
    return when {
        template.onPreApprovalInstantVerificationSummaryTemplate != null -> {
            val uiState = template.onPreApprovalInstantVerificationSummaryTemplate!!.toUiState(
                progress = progress ?: 0.0,
                templateCommon = generalTemplate,
            )
            uiState
        }
        else -> ""
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for long navigation when-branch guard, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_PositiveWhenBranchAlternateCondition(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

data class ErrorPayload(val message: String)
data class Response(val onError: ErrorPayload?)

fun message(response: Response, fallback: String?): String {
    return when {
        response.onError != null, fallback != null -> response.onError!!.message
        else -> ""
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when only one when-entry alternative proves non-null")
	}
}

func TestUnsafeCallOnNullableType_NegativeTextUtilsIsEmptyElseBranch(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

object TextUtils {
    fun isEmpty(value: String?): Boolean = value == null || value.isEmpty()
}

fun normalize(query: String?): String {
    return if (TextUtils.isEmpty(query)) "" else query!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for TextUtils.isEmpty else-branch smart cast, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeShortCircuitNullGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun enabled(flags: Int?): Boolean {
    return flags != null && flags!! and 1 != 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for same-expression && null guard, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeShortCircuitStableMatcherGroup(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
class Matcher {
    fun group(index: Int): String? = null
}
fun scrub(matcher: Matcher): Boolean {
    return matcher.group(1) != null && matcher.group(1)!!.isNotEmpty()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for stable Matcher.group same-expression guard, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeShortCircuitUnresolvedSimpleIdentifier(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
class Message(val flags: Int?) {
    val enabled: Boolean get() = flags != null && flags!! and 1 != 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for same-expression simple identifier guard, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_PositiveShortCircuitRepeatedFunctionCall(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun nextName(): String? = null
fun greet(): Int {
    return if (nextName() != null && nextName()!!.isNotEmpty()) 1 else 0
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for repeated function call in && guard")
	}
}

func TestUnsafeCallOnNullableType_NegativeAndroidCompatAccessors(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

class Intent {
    fun getBundleExtra(key: String): Bundle? = null
    fun <T> getParcelableExtraCompat(key: String, clazz: Class<T>): T? = null
    fun <T> getParcelableArrayListExtraCompat(key: String, clazz: Class<T>): ArrayList<T>? = null
}
class Bundle
class Parcel {
    fun <T> readParcelableCompat(clazz: Class<T>): T? = null
    fun <T> readSerializableCompat(clazz: Class<T>): T? = null
}
class Args

fun read(intent: Intent, parcel: Parcel) {
    val bundle = intent.getBundleExtra("args")!!
    val args = intent.getParcelableExtraCompat("args", Args::class.java)!!
    val list = intent.getParcelableArrayListExtraCompat("args", Args::class.java)!!
    val parcelable = parcel.readParcelableCompat(Args::class.java)!!
    val serializable = parcel.readSerializableCompat(Args::class.java)!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for Android compat accessor assertions, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeProtoNestedBangChain(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import com.example.protos.push.SyncMessage

class GroupV2
class Message(val groupV2: GroupV2?)
class Sent(val message: Message?)
class SyncMessage(val sent: Sent?)
class DataMessage(val flags: Int?)

fun group(syncMessage: SyncMessage): GroupV2 {
    return syncMessage.sent!!.message!!.groupV2!!
}
val DataMessage.hasFlag: Boolean get() = flags!! and 1 != 0
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for proto nested !! field chain, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_PositiveUnqualifiedProtoFieldOutsideExtensionReceiver(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import com.example.protos.push.DataMessage

fun hasFlag(flags: Int?): Boolean {
    return flags!! and 1 != 0
}
`)
	if len(findings) == 0 {
		t.Fatal("expected unqualified proto-like local outside an extension receiver to be flagged")
	}
}

func TestUnsafeCallOnNullableType_PositiveUnqualifiedProtoLikeFieldOutsideProtoFile(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun hasFlag(flags: Int?): Boolean {
    return flags!! and 1 != 0
}
`)
	if len(findings) == 0 {
		t.Fatal("expected ordinary unqualified proto-like field name outside proto imports to be flagged")
	}
}

func TestUnsafeCallOnNullableType_NegativeRequireFunctionBody(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
class State(private val username: String?) {
    fun requireUsername(): String = username!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for require* single-expression body, got %d", len(findings))
	}
}

// Regression: the require* exemption only covers the `!!` in a single-expression
// body (`fun requireXxx(): T = field!!`). A `!!` that appears in a parameter
// default value or anywhere else in a require*-named function must still be
// flagged — the previous text-based `=` heuristic confused the parameter's
// `=` with an expression-body `=` and silently exempted it.
func TestUnsafeCallOnNullableType_PositiveRequireFunctionDefaultArgBang(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
class State(private val username: String?) {
    fun requireUser(name: String = username!!): String {
        return name
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for !! in default argument of require* function, got none")
	}
}

// Regression: a require*-named function with a block body and a default
// argument containing `=` must not be exempted via the parameter `=`.
func TestUnsafeCallOnNullableType_PositiveRequireBlockBodyWithDefaultArg(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
class State(private val username: String?) {
    fun requireUser(retries: Int = 3): String {
        return username!!
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for !! in block body of require* function with default arg, got none")
	}
}

func TestUnsafeCallOnNullableType_NegativeSameBlockConstructorAssignment(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
class Controller {
    fun start() {}
}
class Holder {
    private var controller: Controller? = null
    fun update() {
        if (controller == null) {
            controller = Controller()
        }
        controller!!.start()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for same-block constructor assignment guard, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_PositiveSameBlockNullableFactoryAssignment(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
class Controller {
    fun start() {}
}
fun createController(): Controller? = null
class Holder {
    private var controller: Controller? = null
    fun update() {
        controller = createController()
        controller!!.start()
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when same-block assignment comes from nullable-looking factory")
	}
}

func TestUnsafeCallOnNullableType_PositiveSameBlockOverwrittenWithNull(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
class Controller {
    fun start() {}
}
class Holder {
    private var controller: Controller? = null
    fun update() {
        controller = Controller()
        controller = null
        controller!!.start()
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when same-block non-null assignment is overwritten with null")
	}
}

// --- NullableToStringCall ---

func TestNullableToStringCall_TemplateShapes(t *testing.T) {
	flagged := map[string]string{
		"safe call result": "fun f(x: String?) { val r = \"${x?.length}\" }",
		"plain nullable":   "fun f(x: String?) { val r = \"$x\" }",
	}
	for name, src := range flagged {
		if got := runRuleByNameWithResolver(t, "NullableToStringCall", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	clean := map[string]string{
		"not-null assertion": "fun f(x: String?) { val r = \"${x!!}\" }",
		"elvis fallback":     "fun f(x: String?) { val r = \"${x ?: \"\"}\" }",
	}
	for name, src := range clean {
		if got := runRuleByNameWithResolver(t, "NullableToStringCall", "package test\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
}

func TestNullableToStringCall_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NullableToStringCall", `
package test
fun display(value: Int?) {
    val text = value.toString()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for nullable toString(), got none")
	}
}

func TestNullableToStringCall_PositiveComplexReceiver(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NullableToStringCall", `
package test
class User
class Repo {
    fun findUser(): User? = null
}
fun display(repo: Repo) {
    val text = repo.findUser().toString()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for nullable complex receiver toString(), got none")
	}
}

func TestNullableToStringCall_PositiveMultiline(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NullableToStringCall", `
package test
fun display(value: Int?) {
    val text = value
        .toString()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for multiline nullable toString(), got none")
	}
}

func TestNullableToStringCall_PositiveStringTemplate(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NullableToStringCall", `
package test
fun display(value: Int?) {
    val text = "value=$value"
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for nullable string template interpolation, got none")
	}
}

func TestNullableToStringCall_PositiveResolvedKotlinTarget(t *testing.T) {
	findings := runRuleByNameWithCallTarget(t, "NullableToStringCall", `
package test
fun display(value: Int?) {
    val text = value.toString()
}
`, "value.toString()", "kotlin.toString")
	if len(findings) == 0 {
		t.Fatal("expected finding for oracle-resolved Kotlin toString target, got none")
	}
}

func TestNullableToStringCall_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NullableToStringCall", `
package test
fun display(value: Int) {
    val text = value.toString()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for non-nullable toString(), got %d", len(findings))
	}
}

func TestNullableToStringCall_NegativeSafeCall(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NullableToStringCall", `
package test
fun display(value: Int?) {
    val text = value?.toString()
    val fallback = value?.toString() ?: ""
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe-call toString(), got %d", len(findings))
	}
}

func TestNullableToStringCall_NegativeStringLiteralAndComment(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NullableToStringCall", `
package test
fun display(value: Int?) {
    // value.toString()
    val text = "value.toString()"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for comments or string literals, got %d", len(findings))
	}
}

func TestNullableToStringCall_NegativeCustomNullableExtension(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NullableToStringCall", `
package test
class User
fun User?.toString(): String = ""
fun display(user: User?) {
    val text = user.toString()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for custom nullable toString extension, got %d", len(findings))
	}
}

func TestNullableToStringCall_NegativeResolvedCustomTarget(t *testing.T) {
	findings := runRuleByNameWithCallTarget(t, "NullableToStringCall", `
package test
class User
fun display(user: User?) {
    val text = user.toString()
}
`, "user.toString()", "test.User.toString")
	if len(findings) != 0 {
		t.Fatalf("expected no findings for oracle-resolved custom toString target, got %d", len(findings))
	}
}

func TestNullableToStringCall_NegativeUnresolvedTemplateExpression(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NullableToStringCall", `
package test
fun display() {
    val text = "value=$missing"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for unresolved template expression, got %d", len(findings))
	}
}

// --- NullCheckOnMutableProperty ---

func TestNullCheckOnMutableProperty_Positive(t *testing.T) {
	findings := runRuleByName(t, "NullCheckOnMutableProperty", `
package test
class Foo {
    var name: String? = null
    fun check() {
        if (name != null) {
            println(name)
        }
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for null check on mutable property, got none")
	}
}

func TestNullCheckOnMutableProperty_Negative(t *testing.T) {
	findings := runRuleByName(t, "NullCheckOnMutableProperty", `
package test
class Foo {
    val name: String? = null
    fun check() {
        if (name != null) {
            println(name)
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for null check on immutable val, got %d", len(findings))
	}
}

// --- MapGetWithNotNullAssertionOperator ---

func TestMapGetWithNotNullAssertion_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MapGetWithNotNullAssertionOperator", `
package test
fun lookup(map: Map<String, Int>) {
    val value = map["key"]!!
    val value2 = map.get("key")!!
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings for map access with !!, got %d", len(findings))
	}
}

func TestMapGetWithNotNullAssertion_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MapGetWithNotNullAssertionOperator", `
package test
class Box { operator fun get(key: String): String? = null }
operator fun Map<*, *>.get(one: Int): Int? = null
fun lookup(map: Map<String, Int>) {
    val value = map.getValue("key")
    if (map.containsKey("other")) {
        val guarded = map["other"]!!
    }
    val extensionGet = map[0]!!
}
fun ok(box: Box) {
    val value = box["x"]!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for getValue(), guarded access, or non-map indexing, got %d", len(findings))
	}
}

func TestMapGetWithNotNullAssertion_NestedReceiver(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MapGetWithNotNullAssertionOperator", `
package test
class Holder(val maps: Maps)
class Maps(val current: Map<String, Int>)
fun lookup(holder: Holder, key: String) {
    val value = holder.maps.current[key]!!
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for nested map receiver, got %d", len(findings))
	}
}

func TestMapGetWithNotNullAssertion_DoesNotMatchUnrelatedNestedTerminalName(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MapGetWithNotNullAssertionOperator", `
package test
class Maps(val current: Map<String, Int>)
class Other(val current: Box)
class Box { operator fun get(key: String): String? = null }
fun lookup(other: Other, key: String) {
    val value = other.current[key]!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for unrelated nested terminal name, got %d", len(findings))
	}
}

// --- CastNullableToNonNullableType ---

func TestCastNullableToNonNullableType_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert(obj: String?) {
    val str = obj as String
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for casting nullable to non-nullable, got none")
	}
}

func TestCastNullableToNonNullableType_InferredNullablePositive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert(flag: Boolean): String {
    val value = if (flag) "ok" else null
    return value as String
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for inferred nullable value cast to non-nullable, got none")
	}
}

func TestCastNullableToNonNullableType_MultilineGenericPositive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert(values: List<String>?) {
    val cast =
        (values) as
            List<String>
    println(cast)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for multiline nullable generic cast, got none")
	}
	if findings[0].Fix == nil {
		t.Fatal("expected fix for multiline nullable generic cast")
	}
	if findings[0].Fix.Replacement != "as?" {
		t.Fatalf("expected fix to replace only operator with as?, got %q", findings[0].Fix.Replacement)
	}
}

func TestCastNullableToNonNullableType_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert(obj: Any?) {
    val str = obj as? String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe cast as?, got %d", len(findings))
	}
}

func TestCastNullableToNonNullableType_FollowsDetektShapes(t *testing.T) {
	flagged := map[string]string{
		"safe call source":                "fun f(s: String?) { val j = s?.length as Int }",
		"elvis of nullables":              "fun f(s: String?, t: String?) { val j = (s ?: t) as String }",
		"unbounded type parameter source": "fun <T> f(s: T) { val j = s as String }",
	}
	for name, src := range flagged {
		if got := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	clean := map[string]string{
		"parenthesized safe cast": "fun f(x: Any?) { val j = (x as? String) as String }",
		"chained cast":            "fun f(x: Any?) { val j = x as? String as String }",
		"nullable type parameter": "fun <T> f(s: String?) { val j = s as T }",
	}
	for name, src := range clean {
		if got := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", "package test\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
}

func TestCastNullableToNonNullableType_NegativeNullableTarget(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert(obj: String?) {
    val str = obj as String?
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable target cast, got %d", len(findings))
	}
}

func TestCastNullableToNonNullableType_NegativeNullLiteral(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert() {
    val str = null as String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for null literal cast, got %d", len(findings))
	}
}

func TestCastNullableToNonNullableType_NegativeUnresolvedSource(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert() {
    val str = missing as String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for unresolved source nullability, got %d", len(findings))
	}
}

// Oracle reports nullable LHS → rule fires.
func TestCastNullableToNonNullableType_OracleExpressionTypeMarksSourceNullable(t *testing.T) {
	findings := runCastNullableWithOracleExprType(t, `
package test
fun convert(unknownReceiver: String?): String {
    return unknownReceiver.something() as String
}
`, "unknownReceiver.something()", &typeinfer.ResolvedType{
		Name:     "String",
		FQN:      "kotlin.String",
		Kind:     typeinfer.TypeClass,
		Nullable: true,
	})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when oracle reports nullable source, got %d: %v", len(findings), findings)
	}
}

// Oracle reports non-nullable LHS → no finding.
func TestCastNullableToNonNullableType_OracleExpressionTypeSuppressesNonNullableSource(t *testing.T) {
	findings := runCastNullableWithOracleExprType(t, `
package test
fun convert(unknownReceiver: String?): String {
    return unknownReceiver.something() as String
}
`, "unknownReceiver.something()", &typeinfer.ResolvedType{
		Name:     "String",
		FQN:      "kotlin.String",
		Kind:     typeinfer.TypeClass,
		Nullable: false,
	})
	if len(findings) != 0 {
		t.Fatalf("expected oracle non-nullable source to suppress finding, got %d: %v", len(findings), findings)
	}
}

// Pins NeedsOracleExprType on the rule.
func TestCastNullableToNonNullableType_DeclaresOracleExprType(t *testing.T) {
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "CastNullableToNonNullableType" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("CastNullableToNonNullableType rule not registered")
	}
	if !rule.Needs.Has(api.NeedsOracleExprType) {
		t.Errorf("missing NeedsOracleExprType: Needs=%b", rule.Needs)
	}
}

// runCastNullableWithOracleExprType plants `typ` at the cast LHS
// matching `lhsText` and runs CastNullableToNonNullableType.
func runCastNullableWithOracleExprType(t *testing.T, code string, lhsText string, typ *typeinfer.ResolvedType) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{}
	file.FlatWalkNodes(0, "as_expression", func(idx uint32) {
		var source uint32
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				source = child
				break
			}
		}
		if source == 0 || strings.TrimSpace(file.FlatNodeText(source)) != lhsText {
			return
		}
		key := fmt.Sprintf("%d:%d", file.FlatRow(source)+1, file.FlatCol(source)+1)
		fake.Expressions[file.Path][key] = typ
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == "CastNullableToNonNullableType" {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", "CastNullableToNonNullableType")
	return nil
}

// --- CastToNullableType ---

func TestCastToNullableType_Positive(t *testing.T) {
	findings := runRuleByName(t, "CastToNullableType", `
package test
fun convert(obj: Any) {
    val str = obj as String?
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for 'as String?', got none")
	}
}

func TestCastToNullableType_GenericPositive(t *testing.T) {
	findings := runRuleByName(t, "CastToNullableType", `
package test
fun convert(obj: Any) {
    val list = obj as List<String>?
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for nullable generic target, got none")
	}
	if findings[0].Fix == nil {
		t.Fatal("expected fix for nullable generic target")
	}
	if findings[0].Fix.Replacement != "obj as? List<String>" {
		t.Fatalf("expected structural fix for nullable generic target, got %q", findings[0].Fix.Replacement)
	}
}

func TestCastToNullableType_Negative(t *testing.T) {
	findings := runRuleByName(t, "CastToNullableType", `
package test
fun convert(obj: Any) {
    val str = obj as? String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe cast as?, got %d", len(findings))
	}
}

func TestCastToNullableType_NegativeNestedNullableArgument(t *testing.T) {
	findings := runRuleByName(t, "CastToNullableType", `
package test
class Box<T>
fun convert(obj: Any) {
    val box = obj as Box<String?>
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when only a type argument is nullable, got %d", len(findings))
	}
}

func TestCastToNullableType_NegativeCommentAndStringMentions(t *testing.T) {
	findings := runRuleByName(t, "CastToNullableType", `
package test
fun convert(obj: Any) {
    // obj as String?
    val text = "obj as String?"
    val str = obj as? String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for comment/string cast mentions, got %d", len(findings))
	}
}

// --- UnnecessaryNotNullCheck ---

func TestUnnecessaryNotNullCheck_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
fun check() {
    val name: String = "hello"
    if (name != null) {
        println(name)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unnecessary null check on non-nullable val, got none")
	}
}

func TestUnnecessaryNotNullCheck_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
fun check() {
    val name: String? = null
    if (name != null) {
        println(name)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for null check on nullable val, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullCheck_PositiveResolvedCall(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
fun name(): String = "hello"
fun check() {
    if (name() == null) {
        println("impossible")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for null check on non-nullable call result, got none")
	}
}

func TestUnnecessaryNotNullCheck_NegativeUnresolvedCall(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
fun check() {
    if (missing() != null) {
        println("unknown")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for unresolved call target, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullCheck_NegativeArbitraryExpression(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
fun check(name: String) {
    if (name + "!" != null) {
        println(name)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for arbitrary non-reference expression, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullCheck_PositiveQualifiedSameFileMember(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
class User {
    val name: String = "Ada"
}
fun check(user: User) {
    if (user.name != null) {
        println(user.name)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for non-nullable same-file member null check, got none")
	}
}

func TestUnnecessaryNotNullCheck_NegativeNullableSameFileMember(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
class User {
    val name: String? = null
}
fun check(user: User) {
    if (user.name != null) {
        println(user.name)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable same-file member null check, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullCheck_NegativeUnresolvedConstructorCall(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
fun check() {
    if (MissingType() != null) {
        println("unknown")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for unresolved constructor-like call, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullCheck_NegativeQualifiedCallDoesNotMatchTopLevelFunction(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
class Api
fun fetch(): String = "local"
fun check(api: Api) {
    if (api.fetch() != null) {
        println("unknown receiver call")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for qualified call whose receiver target is unresolved, got %d", len(findings))
	}
}

// --- UnnecessarySafeCall ---

func TestUnnecessarySafeCall_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessarySafeCall", `
package test
fun check() {
    val name: String = "hello"
    val len = name?.length
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unnecessary safe call on non-nullable val, got none")
	}
}

func TestUnnecessarySafeCall_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessarySafeCall", `
package test
fun check() {
    val name: String? = null
    val len = name?.length
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe call on nullable val, got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_NegativeNullableExtensionPropertyReceiver(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class DataMessage(val groupV2: GroupContextV2?)
class GroupContextV2(val masterKey: ByteArray?)

val DataMessage?.hasGroupContext: Boolean
    get() = this?.groupV2?.masterKey.isNotEmpty()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable extension property receiver, got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_NegativeNullableWithReceiverThis(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class Logger(val lastTracked: Tracked?)
class Tracked(val a: A?)
class A(val b: B?)
class B(val event: Event?)
class Event(val id: String, val uiContext: String)

fun render(logger: Logger) {
    with(logger.lastTracked?.a?.b?.event) {
        val id = this?.id
        val uiContext = this?.uiContext
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for this safe calls inside with(nullable), got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_NegativeNullableWithReceiverRepeatedThis(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class Helper(val lastTracking: Event?)
class Event(val eventId: String, val channelId: String)

fun render(helper: Helper) {
    with(helper.lastTracking) {
        val eventId = this?.eventId
        val channelId = this?.channelId
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for repeated this safe calls inside with(nullable property), got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_NegativeNullableRunApplyReceiverThis(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class Logger(val lastTracked: Tracked?)
class Tracked(val event: Event?)
class Event(var id: String)

fun render(logger: Logger) {
    logger.lastTracked?.event.run {
        val id = this?.id
    }
    logger.lastTracked?.event.apply {
        this?.id = "updated"
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for this safe calls inside nullable run/apply receivers, got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_NegativeNullableLetAlsoImplicitIt(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class Logger(val lastTracked: Tracked?)
class Tracked(val event: Event?)
class Event(val id: String)

fun render(logger: Logger) {
    logger.lastTracked?.event.let {
        val id = it?.id
    }
    logger.lastTracked?.event.also {
        val id = it?.id
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for it safe calls inside nullable let/also receivers, got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_PositiveNonNullCanvasParameter(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class Canvas {
    fun draw() {}
}

fun onDraw(canvas: Canvas) {
    canvas?.draw()
}

`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unnecessary safe call on non-null canvas parameter")
	}
}

func TestUnnecessarySafeCall_FunctionParameterResolvedType(t *testing.T) {
	t.Run("generic parameter is not flagged", func(t *testing.T) {
		file := parseInline(t, `
package test

fun <T> f(x: T) {
    x?.hashCode()
}
`)
		resolver := typeinfer.NewFakeResolver()
		resolver.NameTypes["x"] = &typeinfer.ResolvedType{Kind: typeinfer.TypeGeneric, Resolved: true}
		findings := runRuleOnFileWithResolver(t, "UnnecessarySafeCall", file, resolver)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for generic parameter safe call, got %d: %#v", len(findings), findings)
		}
	})

	t.Run("concrete non-null parameter is flagged", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class Known

fun f(x: Known) {
    x?.hashCode()
}
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for concrete non-null parameter safe call")
		}
	})
}

func runRuleOnFileWithResolver(t *testing.T, ruleName string, file *scanner.File, resolver typeinfer.TypeResolver) []scanner.Finding {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == ruleName {
			d := rules.NewDispatcher([]*api.Rule{r}, resolver)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

func TestUnnecessarySafeCall_NegativeNullableInvokeResult(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class Filter
class Input(val enabled: Boolean)
class GetInput {
    operator fun invoke(filter: Filter): Input? = null
}

fun test(filter: Filter) {
    val result = GetInput()(filter)
    result?.enabled
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected nullable invoke result safe call to be ignored, got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_PositiveConstructorInitializedLocal(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class Input(val enabled: Boolean)

fun test() {
    val result = Input(true)
    result?.enabled
}
`)
	if len(findings) == 0 {
		t.Fatal("expected constructor initialized local safe call to be flagged")
	}
}

func TestUnnecessarySafeCall_NegativeOrNullInitializedLocal(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class Input(val enabled: Boolean)

fun test(items: List<Input>) {
    val result = items.firstOrNull()
    result?.enabled
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected OrNull initialized local safe call to be ignored, got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_NegativeExternalReceiverLambdaThis(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class Activity {
    fun finish() {}
}

class Provider {
    fun useForegroundActivity(block: Activity?.() -> Unit) {}
}

fun navigate(provider: Provider) {
    provider.useForegroundActivity {
        this?.finish()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected external receiver lambda this safe call to be ignored, got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_NegativeImportedNullableMember(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

import app.Config.listener

fun test() {
    listener?.log()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected imported member safe call to be ignored, got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_NegativeFrameworkActivityProperty(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class Screen {
    fun close() {
        activity?.finish()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected activity property safe call to be ignored, got %d", len(findings))
	}
}

// --- UnnecessaryNotNullOperator ---

func TestUnnecessaryNotNullOperator_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryNotNullOperator", `
package test
fun check() {
    val name: String = "hello"
    val len = name!!.length
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unnecessary !! on non-nullable val, got none")
	}
}

func TestUnnecessaryNotNullOperator_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryNotNullOperator", `
package test
fun check() {
    val name: String? = null
    val len = name!!.length
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for !! on nullable val, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullOperator_NegativeNullableReceiverInApply(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullOperator", `
package test

class Typeface(val style: Int)
class TextPaint(var typeface: Typeface?)

fun update(tp: TextPaint?) {
    tp.apply {
        val old = this!!.typeface
        println(old)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable receiver in apply, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullOperator_NegativeNullableGenericDocument(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullOperator", `
package test

interface Document<I> {
    val items: MutableList<I>
}

fun <D : Document<I>?, I> consume(document: D) {
    val iterator = document!!.items.iterator()
    println(iterator)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable generic document, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullOperator_NegativeNullableGenericLocalVal(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullOperator", `
package test

interface Document<I> {
    val items: MutableList<I>
}

fun <D : Document<I>?, I> load(input: D): D = input

fun <D : Document<I>?, I> consume(input: D) {
    val document: D = load(input)
    val iterator = document!!.items.iterator()
    println(iterator)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable generic local val, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullOperator_NegativeNullableGenericLocalValInsideLambda(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullOperator", `
package test

interface Document<I> {
    val items: MutableList<I>
}

class Db

fun <T> withinTransaction(block: (Db) -> T): T {
    throw RuntimeException()
}

fun <D : Document<I>?, I> getDocument(db: Db, clazz: Class<D>): D {
    throw RuntimeException()
}

fun <D : Document<I>?, I> consume(clazz: Class<D>) {
    withinTransaction { db ->
        val document: D = getDocument(db, clazz)
        val iterator = document!!.items.iterator()
        println(iterator)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable generic local val inside lambda, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullOperator_NegativeNullableApplyReceiverThis(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullOperator", `
package test

class Typeface(val style: Int)
class TextPaint(var typeface: Typeface?)

fun <T> T.apply(block: T.() -> Unit): T {
    block()
    return this
}

fun update(tp: TextPaint?) {
    tp.apply {
        val old = this!!.typeface
        println(old)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable apply receiver this!!, got %d", len(findings))
	}
}

// --- Regression tests for AST-based locators in nullsafety_redundant ---

// TestUnnecessarySafeCall_FixUsesAstOperatorOffset confirms the autofix
// targets the AST `?.` token's byte range. A regression where the rule used
// strings.Index over the navigation_expression text would corrupt the source
// if the receiver text contained `?.` (e.g. inside a string literal); the
// AST-based locator is immune to that. This test simply pins the fix range
// to the exact bytes of the operator token.
func TestUnnecessarySafeCall_FixUsesAstOperatorOffset(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test
fun example(s: String) {
    val len = s?.length
    println(len)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for s?.length on non-null param")
	}
	var found bool
	for _, f := range findings {
		if f.Fix == nil {
			continue
		}
		if f.Fix.Replacement != "." {
			continue
		}
		span := f.Fix.EndByte - f.Fix.StartByte
		if span != 2 {
			t.Fatalf("fix span = %d, want 2 (length of `?.`)", span)
		}
		found = true
	}
	if !found {
		t.Fatal("expected a fix with replacement `.` and 2-byte span over `?.`")
	}
}

// TestGetterNullableReceiverFlat_BacktickColonBoundary confirms the
// receiver-type prefix scan in getterNullableReceiverFlat is anchored to
// the variable_declaration AST child, not the first `:` in the property
// text. A backtick-quoted property name containing `:` must not shift the
// boundary into the name itself.
func TestGetterNullableReceiverFlat_BacktickColonBoundary(t *testing.T) {
	// The rule under test is UnnecessarySafeCall. The getter helper is
	// invoked when checking `this?.X` inside a property's getter; the
	// preceding property declaration here uses a backtick-quoted name
	// containing `:`. If the boundary leaks into the name, the heuristic
	// is consulted on text that does not contain the receiver-type prefix
	// and the rule's behavior on the getter must remain correct
	// (specifically: the safe-call inside the getter of a non-receiver
	// property must NOT be flagged as unnecessary just because the
	// preceding declaration's text gets misparsed).
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", "\n"+
		"package test\n"+
		"class Foo { fun uppercase(): String = \"\" }\n"+
		"val `foo:bar`: Foo? = null\n"+
		"val String?.thing: String\n"+
		"  get() = this?.uppercase() ?: \"\"\n")
	for _, f := range findings {
		if strings.Contains(f.Message, "this") {
			t.Fatalf("did not expect a finding on this?.uppercase() inside an extension property with nullable receiver; got: %+v", f)
		}
	}
}

func TestUnsafeCast_DiagnosticRequiresKotlinSourceResolvedTypes(t *testing.T) {
	// An unresolved target type is not a source-resolved Kotlin type, so the compiler verdict is not reported.
	unresolved := runRuleByNameWithOracleDiagnostic(t, "UnsafeCast", `
package test
fun process(obj: Any) {
    val v = obj as UnknownLibraryType
}
`, "obj as UnknownLibraryType", "CAST_NEVER_SUCCEEDS")
	if len(unresolved) != 0 {
		t.Fatalf("expected no finding for an unresolved target type, got %d", len(unresolved))
	}
	// A project Kotlin class on both sides still reports.
	resolved := runRuleByNameWithOracleDiagnostic(t, "UnsafeCast", `
package test
class Left
class Right
fun process(left: Left) {
    val v = left as Right
}
`, "left as Right", "CAST_NEVER_SUCCEEDS")
	if len(resolved) != 1 {
		t.Fatalf("expected 1 finding for Kotlin source types, got %d", len(resolved))
	}
}

func TestUnsafeCast_SkipsPlatformCallSource(t *testing.T) {
	findings := runRuleByNameWithOracleDiagnostic(t, "UnsafeCast", `
package test
import android.view.ViewStub
class Holder(private val stub: ViewStub) {
    fun inflateIt() {
        val view = stub.inflate() as CustomBanner
    }
}
`, "stub.inflate() as CustomBanner", "CAST_NEVER_SUCCEEDS")
	if len(findings) != 0 {
		t.Fatalf("expected no finding for a cast from a platform call result, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_PreconditionGuards(t *testing.T) {
	cases := map[string]string{
		"elvis return":    "other ?: return\n    other!!.run()",
		"check":           "check(other != null)\n    other!!.run()",
		"require":         "require(other != null && ready)\n    other!!.run()",
		"requireNotNull":  "requireNotNull(other)\n    other!!.run()",
		"checkNotNull":    "checkNotNull(other) { \"missing\" }\n    other!!.run()",
		"val elvis throw": "val x = other ?: throw IllegalStateException()\n    other!!.run()",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			findings := runRuleByName(t, "UnsafeCallOnNullableType", "package test\n\nclass A {\n  var other: Foo? = null\n  fun f(ready: Boolean) {\n    "+body+"\n  }\n}\n")
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestUnsafeCallOnNullableType_UnrelatedPreconditionStillReports(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

class A {
  var other: Foo? = null
  var third: Foo? = null
  fun f() {
    requireNotNull(third)
    other!!.run()
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_NoOracleKotlinTestAssertNotNullDoesNotSmartCast(t *testing.T) {
	findings := runRuleByName(t, "UnnecessarySafeCall", `
package test

import kotlin.test.assertNotNull

class State(val workspaces: List<String>?)

fun check(next: () -> State?, state: State) {
    var latest: State? = next()
    assertNotNull(latest)
    println(latest?.workspaces)
    assertNotNull(state.workspaces)
    println(state.workspaces?.size)
}
`)
	if len(findings) != 0 {
		t.Fatalf("source-only path must not infer a smart cast from kotlin.test.assertNotNull, got %v", findings)
	}
}

func TestUnsafeCallOnNullableType_GenericFieldNamesOnlyExemptWithProtoImports(t *testing.T) {
	plain := "package test\nclass U(val id: String?, val value: Int?)\nfun f(u: U) {\n    println(u.id!!)\n    println(u.value!!)\n}\n"
	if got := runRuleByName(t, "UnsafeCallOnNullableType", plain); len(got) != 2 {
		t.Errorf("ordinary id/value fields must be reported, got %d", len(got))
	}
	proto := "package test\nimport com.squareup.wire.Message\nclass U(val id: String?, val value: Int?)\nfun f(u: U) {\n    println(u.id!!)\n    println(u.value!!)\n}\n"
	if got := runRuleByName(t, "UnsafeCallOnNullableType", proto); len(got) != 0 {
		t.Errorf("proto field reads stay exempt in files that import Wire, got %d", len(got))
	}
}

func TestNullCheckOnMutableProperty_LocalVarsAreSmartCastable(t *testing.T) {
	local := "package test\nfun top(p: String?) {\n    var v: String? = p\n    if (v != null) { println(v.length) }\n}\n"
	if got := runRuleByNameWithResolver(t, "NullCheckOnMutableProperty", local); len(got) != 0 {
		t.Errorf("a local var can be smart-cast, got %v", got)
	}
	member := "package test\nclass B {\n    var s: String? = null\n    fun a() { if (s != null) { println(s.length) } }\n}\n"
	if got := runRuleByNameWithResolver(t, "NullCheckOnMutableProperty", member); len(got) != 1 {
		t.Errorf("a mutable member property must still be reported, got %d", len(got))
	}
}

func TestNullSafety_ConstructorVarIsNotSmartCast(t *testing.T) {
	src := `
package test
class Holder(var p: String?, val q: String?) {
    fun f() {
        if (p != null) { p!!.length; p?.length }
    }
}
`
	for _, rule := range []string{"UnnecessaryNotNullOperator", "UnnecessarySafeCall"} {
		if got := runRuleByName(t, rule, src); len(got) != 0 {
			t.Fatalf("%s: constructor var must not be treated as smart cast, got %d findings", rule, len(got))
		}
	}
}

// runRuleByNameWithExpressionTypes runs a rule with an oracle that reports the given nullability for the
// first expression whose text equals each key.
func runRuleByNameWithExpressionTypes(t *testing.T, ruleName, code string, nullable map[string]bool) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{}
	for text, isNullable := range nullable {
		found := false
		for i := range file.FlatTree.Types {
			idx := uint32(i)
			if found || strings.TrimSpace(file.FlatNodeText(idx)) != text {
				continue
			}
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			rt := &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass, Nullable: isNullable, Resolved: true}
			if isNullable {
				rt.Kind = typeinfer.TypeNullable
			}
			fake.Expressions[file.Path][key] = rt
			found = true
		}
		if !found {
			t.Fatalf("expression %q not found", text)
		}
	}
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == ruleName {
			cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

func TestUnsafeCallOnNullableType_SkipsOperandTheCompilerTypesAsNonNull(t *testing.T) {
	const src = `
package test
fun f(dir: Dir, name: Name) {
    val a = dir.entries()!!
    val b = name.value()!!
}
`
	got := runRuleByNameWithExpressionTypes(t, "UnsafeCallOnNullableType", src,
		map[string]bool{"dir.entries()": false, "name.value()": true})
	if len(got) != 1 || got[0].Line != 5 {
		t.Fatalf("want only the nullable operand on line 5 reported (a platform type is not nullable), got %+v", got)
	}
}

func TestUnsafeCallOnNullableType_ReportsEveryBangBangWithoutOracleFact(t *testing.T) {
	const src = `
package test
fun f(dir: Dir) {
    val a = dir.entries()!!
}
`
	got := runRuleByNameWithResolver(t, "UnsafeCallOnNullableType", src)
	if len(got) != 1 {
		t.Fatalf("want the !! reported when the compiler has no fact for the operand, got %+v", got)
	}
}

func TestUnsafeCallOnNullableType_FlowGuardDoesNotHideUnstableOperand(t *testing.T) {
	const src = `
package test
class Holder {
    var name: String? = null
    fun f() {
        if (name != null) {
            val c = name!!
        }
    }
}
`
	// Without a compiler fact the null check is read as a smart cast.
	if got := runRuleByNameWithResolver(t, "UnsafeCallOnNullableType", src); len(got) != 0 {
		t.Fatalf("want the guarded !! left alone without a compiler fact, got %+v", got)
	}
	// A `var` property is never smart-cast: the compiler types the operand as nullable and the !! is needed.
	got := runRuleByNameWithExpressionTypes(t, "UnsafeCallOnNullableType", src, map[string]bool{"name!!": true})
	if len(got) != 1 {
		t.Fatalf("want the !! on the unstable property reported, got %+v", got)
	}
}

func TestUnsafeCallOnNullableType_IdiomSkipsYieldToCompilerNullableVerdict(t *testing.T) {
	const src = `
package test
fun f(args: Bundle) {
    val a = args.getString("k")!!
}
`
	// Without a compiler fact the Bundle getter idiom stays quiet.
	if got := runRuleByNameWithResolver(t, "UnsafeCallOnNullableType", src); len(got) != 0 {
		t.Fatalf("want the Bundle getter idiom skipped without a compiler fact, got %+v", got)
	}
	// detekt reports any operand whose compiler type is nullable, idiomatic or not.
	got := runRuleByNameWithExpressionTypes(t, "UnsafeCallOnNullableType", src, map[string]bool{`args.getString("k")`: true})
	if len(got) != 1 {
		t.Fatalf("want the nullable Bundle getter operand reported, got %+v", got)
	}
}

func TestUnsafeCallOnNullableType_SmartCastOperandIsNotReported(t *testing.T) {
	const src = `
package test
fun f(name: Name) {
    val c = name.value()!!
}
`
	// The compiler records the smart-cast type; UnnecessaryNotNullOperator owns this shape.
	got := runRuleByNameWithExpressionTypes(t, "UnsafeCallOnNullableType", src, map[string]bool{"name.value()": false})
	if len(got) != 0 {
		t.Fatalf("want no findings for an operand the compiler types as non-null, got %+v", got)
	}
}

func TestUnnecessaryNotNullCheck_PreconditionOnNonNullParameter(t *testing.T) {
	flagged := map[string]string{
		"requireNotNull": "fun f(value: String) { requireNotNull(value) }",
		"checkNotNull":   "fun f(value: Int) { checkNotNull(value) }",
		"with message":   "fun f(value: Foo) { requireNotNull(value) { \"never\" } }",
		"assigned":       "fun f(value: Foo) { val v = requireNotNull(value) }",
		"constructor":    "class A { constructor(value: String) { requireNotNull(value) } }",
	}
	for name, src := range flagged {
		got := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", "package test\n"+src+"\n")
		if len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
			continue
		}
		if !strings.HasPrefix(got[0].Message, "Using `") || !strings.HasSuffix(got[0].Message, "is unnecessary") {
			t.Errorf("%s: message %q is not detekt's", name, got[0].Message)
		}
	}
	clean := map[string]string{
		"nullable parameter":       "fun f(value: String?) { requireNotNull(value) }",
		"nullable generic bound":   "fun <T : Any?> f(value: T) { requireNotNull(value) }",
		"inferred local":           "fun f() { val v = compute(); requireNotNull(v) }",
		"shadowed by nullable":     "fun f(value: String) { val other = value; fun g(value: String?) { requireNotNull(value) } }",
		"call result":              "fun f(value: String) { requireNotNull(value.firstOrNull()) }",
		"member call":              "fun f(value: String) { obj.requireNotNull(value) }",
		"unrelated function":       "fun f(value: String) { requireNoNulls(value) }",
		"property not a parameter": "class A { val value: String = \"\"; fun f() { requireNotNull(value) } }",
	}
	for name, src := range clean {
		if got := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", "package test\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d (%s)", name, len(got), got[0].Message)
		}
	}
}

func TestNullCheckOnMutableProperty_FollowsDetekt(t *testing.T) {
	flagged := map[string]string{
		"member var read in body":    "class A { var p: String? = null\n fun f() { if (p != null) { println(p) } } }",
		"constructor var":            "class A(var p: String?) { fun f() { if (p != null) { println(p) } } }",
		"lateinit var":               "class A { lateinit var p: String\n fun f() { if (p != null) { println(p) } } }",
		"custom getter":              "class A { val p: String? get() = null\n fun f() { if (p != null) { println(p) } } }",
		"top level var":              "var p: String? = null\nfun f() { if (p != null) { println(p) } }",
		"companion var":              "class A { companion object { var p: String? = null }\n fun f() { if (p != null) { println(p) } } }",
		"reversed null comparison":   "class A { var p: String? = null\n fun f() { if (null != p) { println(p) } } }",
		"read in the same condition": "class A { var p: String? = null\n fun f() { if (p != null && p!!.length > 1) { } } }",
	}
	for name, src := range flagged {
		if got := runRuleByName(t, "NullCheckOnMutableProperty", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	clean := map[string]string{
		"val property":          "class A { val p: String? = null\n fun f() { if (p != null) { println(p) } } }",
		"equality check":        "class A { var p: String? = null\n fun f() { if (p == null) return\n println(p) } }",
		"no read in the if":     "class A { var p: String? = null\n fun f() { if (p != null) { println(1) } } }",
		"not in an if":          "class A { var p: String? = null\n fun f() = p != null }",
		"shadowing parameter":   "class A { var p: String? = null\n fun f(p: String?) { if (p != null) { println(p) } } }",
		"shadowing local":       "class A { var p: String? = null\n fun f() { val p = \"x\"\n if (p != null) { println(p) } } }",
		"qualified receiver":    "class A { var p: String? = null\n fun f(o: A) { if (o.p != null) { println(o.p) } } }",
		"non-property variable": "fun f() { var p: String? = null\n if (p != null) { println(p) } }",
	}
	for name, src := range clean {
		if got := runRuleByName(t, "NullCheckOnMutableProperty", "package test\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
	got := runRuleByName(t, "NullCheckOnMutableProperty", "package test\nclass A { var p: String? = null\n fun f() { if (p != null) { println(p) } } }\n")
	if len(got) == 1 && got[0].Message != "Null-check is being called on mutable property 'test.A.p'." {
		t.Errorf("unexpected message %q", got[0].Message)
	}
}
