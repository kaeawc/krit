package rules_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	rulespkg "github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
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
	for _, r := range v2rules.Registry {
		if r.ID == "ForbiddenMethodCall" {
			cols := rulespkg.NewDispatcherV2([]*v2rules.Rule{r}, composite).Run(file)
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

// --- ForbiddenSuppress ---

func TestForbiddenSuppress_SuppressAnnotation(t *testing.T) {
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
