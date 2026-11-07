package rules_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/oracle"
	rulespkg "github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// --- ForbiddenComment: FIXME and STOPSHIP edge cases ---

func TestForbiddenComment_StyleFIXME(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `
package test
// FIXME: this is broken
fun example() {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenComment" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ForbiddenComment to flag FIXME: in a comment")
	}
}

func TestForbiddenComment_StyleSTOPSHIP(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `
package test
// STOPSHIP: do not ship this
fun example() {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenComment" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ForbiddenComment to flag STOPSHIP: in a comment")
	}
}

func TestForbiddenComment_IgnoresTrackedBugTODO(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `
package test
@Suppress("RememberReturnType") // TODO: b/372566999
fun example() {}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenComment" {
			t.Fatalf("expected tracked bug TODO to be allowed, got %+v", f)
		}
	}
}

func TestForbiddenComment_FlagsUntrackedTODO(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `
package test
// TODO: clean this up
fun example() {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenComment" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ForbiddenComment to flag an untracked TODO")
	}
}

func TestForbiddenComment_StyleCleanComment(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `
package test
// This is a normal comment
fun example() {}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenComment" {
			t.Error("Should NOT flag a clean comment without forbidden markers")
		}
	}
}

func TestForbiddenComment_JavaLineAndBlockCommentsNotReported(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ForbiddenComment", `
package test;
class Example {
  // FIXME: clean this up
  void run() {
    /* STOPSHIP: debug path */
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt analyzes only Kotlin, expected no Java findings, got %d", len(findings))
	}
}

func TestForbiddenComment_JavaIgnoresStringLiteral(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ForbiddenComment", `
package test;
class Example {
  String run() {
    return "TODO: this is data";
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no Java forbidden-comment findings for string literal, got %d", len(findings))
	}
}

func TestForbiddenComment_KDocContinuationMarker(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `
package test
/**
 * TODO: document this API
 */
fun example() {}
`)
	if len(findings) != 1 {
		t.Fatalf("expected KDoc continuation TODO to be flagged, got %d", len(findings))
	}
}

func TestForbiddenComment_InlineBlockComment(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `
package test
fun example() {
    /* TODO: inline cleanup */ println("done")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected inline block TODO to be flagged, got %d", len(findings))
	}
}

func TestForbiddenComment_BlockCommentDoesNotOvermatchFollowingCode(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `
package test
fun example() {
    /* ordinary comment */ val marker = "TODO: data"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected block comment matcher to ignore following code, got %d", len(findings))
	}
}

// --- WildcardImport ---

func TestWildcardImport_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WildcardImport", `
package test;
import com.example.api.*;
class Example {}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

func TestWildcardImport_JavaSkipsJavaUtilDefaultExclude(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WildcardImport", `
package test;
import java.util.*;
class Example {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no Java wildcard-import finding for java.util default exclude, got %d", len(findings))
	}
}

// --- ForbiddenImport ---

func TestForbiddenImport_SunPackage(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenImport", `
package test
import sun.misc.Unsafe
fun example() {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenImport" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ForbiddenImport to flag 'sun.misc.Unsafe'")
	}
}

func TestForbiddenImport_JdkInternalPackage(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenImport", `
package test
import jdk.internal.misc.SharedSecrets
fun example() {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenImport" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ForbiddenImport to flag 'jdk.internal.' import")
	}
}

func TestForbiddenImport_AllowedImport(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenImport", `
package test
import kotlin.collections.List
fun example() {}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenImport" {
			t.Errorf("Should NOT flag allowed import, got: %s", f.Message)
		}
	}
}

func TestForbiddenImport_JavaSunPackage(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ForbiddenImport", `
package test;
import sun.misc.Unsafe;
class Example {}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

func TestForbiddenImport_JavaAllowedImport(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ForbiddenImport", `
package test;
import java.util.List;
class Example {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no Java forbidden-import finding for allowed import, got %d", len(findings))
	}
}

// --- ForbiddenMethodCall ---

func runForbiddenMethodCallWithTargets(t *testing.T, code string, targets map[string]string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		callText := strings.TrimSpace(file.FlatNodeText(idx))
		for needle, target := range targets {
			if strings.Contains(callText, needle) {
				key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
				fake.CallTargets[file.Path][key] = target
			}
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == "ForbiddenMethodCall" {
			cols := rulespkg.NewDispatcher([]*api.Rule{r}, composite).Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("rule ForbiddenMethodCall not found in registry")
	return nil
}

func TestForbiddenMethodCall_Println(t *testing.T) {
	findings := runForbiddenMethodCallWithTargets(t, `
	package test
	fun example() {
	    println("hello")
	}
	`, map[string]string{"println": "kotlin.io.println"})
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenMethodCall" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ForbiddenMethodCall to flag println()")
	}
}

func TestForbiddenMethodCall_Print(t *testing.T) {
	findings := runForbiddenMethodCallWithTargets(t, `
	package test
	fun example() {
	    print("hello")
	}
	`, map[string]string{"print": "kotlin.io.print"})
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenMethodCall" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ForbiddenMethodCall to flag print()")
	}
}

func TestForbiddenMethodCall_AllowedMethod(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenMethodCall", `
package test
fun example() {
    listOf(1, 2, 3)
}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenMethodCall" {
			t.Errorf("Should NOT flag allowed method call, got: %s", f.Message)
		}
	}
}

func TestForbiddenMethodCall_LocalPrintlnResolvedTarget(t *testing.T) {
	findings := runForbiddenMethodCallWithTargets(t, `
	package test
	fun println(message: String) {}
	fun example() {
	    println("hello")
	}
	`, map[string]string{"println": "test.println"})
	for _, f := range findings {
		if f.Rule == "ForbiddenMethodCall" {
			t.Errorf("Should NOT flag a local println declaration with a resolved non-forbidden target, got: %s", f.Message)
		}
	}
}

// --- ForbiddenAnnotation ---

func TestForbiddenAnnotation_SuppressWarnings(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenAnnotation", `
package test
@SuppressWarnings("unchecked")
fun example() {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenAnnotation" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ForbiddenAnnotation to flag @SuppressWarnings")
	}
}

func TestForbiddenAnnotation_AllowedAnnotation(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenAnnotation", `
package test
@Override
fun example() {}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenAnnotation" {
			t.Errorf("Should NOT flag allowed annotation, got: %s", f.Message)
		}
	}
}

func TestForbiddenAnnotation_DoesNotMatchStringArgument(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenAnnotation", `
package test
@Deprecated("@SuppressWarnings is mentioned in data")
fun example() {}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenAnnotation" {
			t.Fatalf("ForbiddenAnnotation should not match annotation argument text, got: %s", f.Message)
		}
	}
}

func TestForbiddenAnnotation_DoesNotMatchKDocTag(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenAnnotation", `
package test
/**
 * @SuppressWarnings is documentation, not an annotation.
 */
fun example() {}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenAnnotation" {
			t.Fatalf("ForbiddenAnnotation should not match KDoc tags, got: %s", f.Message)
		}
	}
}

func TestForbiddenAnnotation_FQNConfigRequiresMatchingImport(t *testing.T) {
	rulespkg.ApplyConfig(loadTempConfig(t, `
style:
  ForbiddenAnnotation:
    active: true
    annotations: ["androidx.annotation.RequiresApi"]
`))
	t.Cleanup(func() { rulespkg.ApplyConfig(config.NewConfig()) })

	findings := runRuleByName(t, "ForbiddenAnnotation", `
package test
annotation class RequiresApi
@RequiresApi
fun local() {}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenAnnotation" {
			t.Fatalf("ForbiddenAnnotation should not match same-named local annotations for FQN config, got: %s", f.Message)
		}
	}

	findings = runRuleByName(t, "ForbiddenAnnotation", `
package test
import androidx.annotation.RequiresApi
@RequiresApi
fun imported() {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenAnnotation" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected ForbiddenAnnotation to match imported FQN annotation")
	}
}

// --- ForbiddenComment: detekt anchors and messages ---

func TestForbiddenComment_ReportsEveryMarkerWithDetektMessage(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `package test
// FIXME: and TODO: both
fun f() {}
`)
	if len(findings) != 2 {
		t.Fatalf("expected one finding per marker, got %d: %v", len(findings), findings)
	}
	got := map[string]bool{findings[0].Message: true, findings[1].Message: true}
	for _, want := range []string{
		"Forbidden FIXME todo marker in comment, please fix the problem.",
		"Forbidden TODO todo marker in comment, please do the changes.",
	} {
		if !got[want] {
			t.Errorf("missing message %q in %v", want, got)
		}
	}
}

func TestForbiddenComment_KDocChecksOnlyTheDefaultSectionAtItsStart(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `package test
/**
 * Intro text.
 * TODO: later
 * @param x STOPSHIP: tag
 */
fun f(x: Int) {}

/**
 * @param x STOPSHIP: only in a tag
 */
fun g(x: Int) {}
`)
	if len(findings) != 1 {
		t.Fatalf("expected only the default section to be checked, got %d: %v", len(findings), findings)
	}
	if findings[0].Line != 3 || findings[0].Col != 2 {
		t.Errorf("expected the finding at the start of the default section (3:2), got %d:%d", findings[0].Line, findings[0].Col)
	}
}

// --- ForbiddenSuppress ---

func configureForbiddenSuppress(t *testing.T, rules string) {
	t.Helper()
	rulespkg.ApplyConfig(loadTempConfig(t, "style:\n  ForbiddenSuppress:\n    active: true\n    rules: "+rules+"\n"))
	t.Cleanup(func() { rulespkg.ApplyConfig(config.NewConfig()) })
}

func TestForbiddenSuppress_ReportsNothingWithoutConfiguredRules(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenSuppress", `
package test
@Suppress("UNCHECKED_CAST")
fun example() {}
`)
	if len(findings) != 0 {
		t.Errorf("an empty rules list forbids nothing, got %v", findings)
	}
}

func TestForbiddenSuppress_MessageNamesTheForbiddenRules(t *testing.T) {
	configureForbiddenSuppress(t, `["LongMethod", "MagicNumber"]`)
	findings := runRuleByName(t, "ForbiddenSuppress", `
package test
@Suppress("LongMethod", "MagicNumber", "Other")
fun a() {}
@SuppressWarnings("LongMethod")
fun b() {}
@Suppress("detekt.LongMethod")
fun c() {}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
	}
	msgs := map[string]bool{findings[0].Message: true, findings[1].Message: true}
	for _, want := range []string{
		`Cannot @Suppress rules "LongMethod", "MagicNumber" due to the current configuration.`,
		`Cannot @Suppress rule "LongMethod" due to the current configuration.`,
	} {
		if !msgs[want] {
			t.Errorf("missing message %q in %v", want, msgs)
		}
	}
}

func TestForbiddenSuppress_SuppressAnnotation(t *testing.T) {
	configureForbiddenSuppress(t, `["UNCHECKED_CAST"]`)
	findings := runRuleByName(t, "ForbiddenSuppress", `
package test
@Suppress("UNCHECKED_CAST")
fun example() {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenSuppress" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ForbiddenSuppress to flag @Suppress annotation")
	}
}

func TestForbiddenSuppress_NoSuppressAnnotation(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenSuppress", `
package test
@Deprecated("use newMethod instead")
fun example() {}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenSuppress" {
			t.Errorf("Should NOT flag non-Suppress annotation, got: %s", f.Message)
		}
	}
}

// --- ForbiddenNamedParam ---

func TestForbiddenNamedParam_Positive(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenNamedParam", `
package test
fun example() {
    require(value = true)
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenNamedParam" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ForbiddenNamedParam to flag named argument in require() call")
	}
}

func TestForbiddenNamedParam_Negative(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenNamedParam", `
package test
fun example() {
    require(x > 0)
}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenNamedParam" {
			t.Errorf("Should NOT flag require() without named param, got: %s", f.Message)
		}
	}
}

// --- ForbiddenOptIn ---

func TestForbiddenOptIn_Positive(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenOptIn", `
package test
@OptIn(ExperimentalCoroutinesApi::class)
fun example() {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenOptIn" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ForbiddenOptIn to flag @OptIn annotation")
	}
}

func TestForbiddenOptIn_Negative(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenOptIn", `
package test
@Deprecated("use something else")
fun example() {}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenOptIn" {
			t.Errorf("Should NOT flag non-OptIn annotation, got: %s", f.Message)
		}
	}
}

// --- ForbiddenVoid ---

func TestForbiddenVoid_Positive(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenVoid", `
package test
fun example(): Void {
    return null
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenVoid" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ForbiddenVoid to flag Void return type")
	}
}

func TestForbiddenVoid_Negative(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenVoid", `
package test
fun example(): Unit {
    println("hello")
}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenVoid" {
			t.Errorf("Should NOT flag Unit return type, got: %s", f.Message)
		}
	}
}

func TestForbiddenVoid_HonorsIgnoreUsageInGenerics(t *testing.T) {
	// IgnoreUsageInGenerics was previously a dead config — exposed in
	// metadata but never consulted. Configure it via the rule pointer
	// and verify Void inside *any* type-argument list is skipped, not
	// just the hardcoded javaInteropGenericTypes allowlist.
	var rule *rulespkg.ForbiddenVoidRule
	for _, candidate := range api.Registry {
		if candidate.ID == "ForbiddenVoid" {
			var ok bool
			rule, ok = candidate.Implementation.(*rulespkg.ForbiddenVoidRule)
			if !ok {
				t.Fatalf("expected ForbiddenVoidRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("ForbiddenVoid rule not registered")
	}
	original := rule.IgnoreUsageInGenerics
	defer func() { rule.IgnoreUsageInGenerics = original }()

	// User-defined generic with Void argument — not on the interop allowlist.
	customGenericCode := `package test
class Box<T>
val b: Box<Void> = Box()
`
	// Default behavior (false): the user_type "Void" inside Box<Void> IS flagged.
	if findings := runRuleByName(t, "ForbiddenVoid", customGenericCode); len(findings) == 0 {
		t.Fatal("expected finding for Void in Box<Void> under default IgnoreUsageInGenerics=false")
	}

	rule.IgnoreUsageInGenerics = true

	if findings := runRuleByName(t, "ForbiddenVoid", customGenericCode); len(findings) != 0 {
		t.Fatalf("expected no findings for Void in Box<Void> under IgnoreUsageInGenerics=true, got %d", len(findings))
	}

	// Bare `Void` (not in generics) should still fire under either setting.
	bareVoid := `package test
fun example(): Void = TODO()
`
	if findings := runRuleByName(t, "ForbiddenVoid", bareVoid); len(findings) == 0 {
		t.Fatal("expected finding for bare Void return even with IgnoreUsageInGenerics=true")
	}
}

// --- MandatoryBracesLoops ---

func TestMandatoryBracesLoops_ForNegative_WithBraces(t *testing.T) {
	findings := runRuleByName(t, "MandatoryBracesLoops", `
package test
fun example() {
    val list = listOf(1, 2, 3)
    for (i in list) {
        println(i)
    }
}
`)
	for _, f := range findings {
		if f.Rule == "MandatoryBracesLoops" {
			t.Errorf("Should NOT flag for loop with braces, got: %s", f.Message)
		}
	}
}

func TestMandatoryBracesLoops_WhilePositive(t *testing.T) {
	findings := runRuleByName(t, "MandatoryBracesLoops", `
package test
fun example() {
    var x = 0
    while (x < 10)
        x++
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "MandatoryBracesLoops" {
			found = true
		}
	}
	if !found {
		t.Error("Expected MandatoryBracesLoops to flag while loop without braces")
	}
}

func TestMandatoryBracesLoops_Negative(t *testing.T) {
	findings := runRuleByName(t, "MandatoryBracesLoops", `
package test
fun example() {
    for (i in 1..10) {
        println(i)
    }
}
`)
	for _, f := range findings {
		if f.Rule == "MandatoryBracesLoops" {
			t.Errorf("Should NOT flag loop with braces, got: %s", f.Message)
		}
	}
}

func TestForbiddenVoid_NegativeJavaLibraryGenerics(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenVoid", `
package test
import android.os.OutcomeReceiver
import java.util.concurrent.Callable
import java.util.concurrent.Future

fun receiver(): OutcomeReceiver<Void, Exception>? = null
fun pending(): Future<Void>? = null
fun work(): Callable<Void>? = null
fun task(): Task<Void>? = null
fun listener(): OnSuccessListener<Void>? = null
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for Void as a Java generic type argument, got %d", len(findings))
	}
}

func TestForbiddenImport_DetektImportsGlobsAndPatterns(t *testing.T) {
	rulespkg.ApplyConfig(loadTempConfig(t, `
style:
  ForbiddenImport:
    active: true
    imports: ["wild.card.*", "exact.Name"]
    forbiddenPatterns: 'regex\.pat\..*'
`))
	t.Cleanup(func() { rulespkg.ApplyConfig(config.NewConfig()) })

	findings := runRuleByName(t, "ForbiddenImport", `
package test
import wild.card.Thing
import wild.cardx.Other
import exact.Name
import exact.Name2
import regex.pat.Deep
import exact.Name as Aliased
import com.assun.Thing
`)
	got := 0
	for _, f := range findings {
		if f.Rule == "ForbiddenImport" {
			got++
		}
	}
	if got != 4 {
		t.Fatalf("expected 4 findings (wild.card.Thing, exact.Name, regex.pat.Deep, aliased exact.Name), got %d: %v", got, findings)
	}
}

func applySingleFix(t *testing.T, src string, f scanner.Finding) string {
	t.Helper()
	if f.Fix == nil {
		t.Fatalf("finding %q has no fix", f.Message)
	}
	return src[:f.Fix.StartByte] + f.Fix.Replacement + src[f.Fix.EndByte:]
}

func TestForbiddenSuppress_FixOnlyRemovesTheAnnotation(t *testing.T) {
	configureForbiddenSuppress(t, `["x", "y", "z"]`)
	src := "package test\n\nclass A(@Suppress(\"x\") val b: Int) {\n  @Suppress(\"y\") fun f() {}\n\n  @Suppress(\"z\")\n  fun g() {}\n}\n"
	findings := runRuleByName(t, "ForbiddenSuppress", src)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(findings))
	}
	wants := []string{
		"class A(val b: Int) {\n  @Suppress(\"y\")",
		"  fun f() {}\n\n  @Suppress(\"z\")",
		"{}\n\n  fun g() {}\n}",
	}
	for i, f := range findings {
		if got := applySingleFix(t, src, f); !strings.Contains(got, wants[i]) {
			t.Errorf("finding %d: fixed source %q does not contain %q", i, got, wants[i])
		}
	}
}

func TestForbiddenOptIn_MarkerClassesMatchClassLiteralShapes(t *testing.T) {
	rulespkg.ApplyConfig(loadTempConfig(t, `
style:
  ForbiddenOptIn:
    active: true
    markerClasses: ["ExperimentalStuff", "kotlin.ExperimentalStdlibApi"]
`))
	t.Cleanup(func() { rulespkg.ApplyConfig(config.NewConfig()) })

	findings := runRuleByName(t, "ForbiddenOptIn", `
package test
@OptIn(ExperimentalStuff::class)
fun a() {}
@OptIn(kotlin.ExperimentalStdlibApi::class)
fun b() {}
@OptIn(OtherMarker::class)
fun c() {}
`)
	if len(findings) != 2 {
		t.Fatalf("expected the two configured markers to be reported, got %d: %v", len(findings), findings)
	}
}
