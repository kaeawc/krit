package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// runUnsafeCastOnProject indexes every file as one project and returns the UnsafeCast findings
// reported in the file named target.
func runUnsafeCastOnProject(t *testing.T, files map[string]string, target string) []scanner.Finding {
	t.Helper()
	dir := t.TempDir()
	var parsed []*scanner.File
	var targetFile *scanner.File
	for name, code := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, f)
		if name == target {
			targetFile = f
		}
	}
	if targetFile == nil {
		t.Fatalf("target %q not among files", target)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel(parsed, 1)
	for _, r := range api.Registry {
		if r.ID == "UnsafeCast" {
			cols := rules.NewDispatcher([]*api.Rule{r}, resolver).Run(targetFile)
			return cols.Findings()
		}
	}
	t.Fatal("UnsafeCast rule not found in registry")
	return nil
}

// Issue #1115, cause 1: a nested `Slot.Context` in an unrelated package must not shadow the
// explicitly imported `android.content.Context`.
func TestUnsafeCast_ImportedTypeNotShadowedByUnrelatedNestedClass(t *testing.T) {
	files := map[string]string{
		"demo/other/Slots.kt": `package demo.other

sealed interface Slot {
    data class Context(val label: String) : Slot
}
`,
		"demo/app/DemoApp.kt": `package demo.app

import android.app.Application

interface AppDelegate { fun registry(): String }

class DemoApp : Application(), AppDelegate {
    override fun registry(): String = "r"
}
`,
		"demo/feature/Provider.kt": `package demo.feature

import android.content.Context
import demo.app.DemoApp

class RegistrarProvider(private val context: Context) {
    fun registry(): String {
        val app = context as DemoApp
        return app.registry()
    }

    fun maybe(): DemoApp? = context as? DemoApp
}
`,
	}
	if findings := runUnsafeCastOnProject(t, files, "demo/feature/Provider.kt"); len(findings) != 0 {
		t.Fatalf("explicit import android.content.Context must win over demo.other.Slot.Context, got %d findings: %+v", len(findings), findings)
	}
}

// Issue #1115, cause 2: `is` narrowing from the then-branch must not leak into the else-branch.
func TestUnsafeCast_IsNarrowingDoesNotLeakIntoElse(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test

sealed class Meta
data class Single(val type: String) : Meta()
data class Multi(val type: String) : Meta()

class SelectPresenter(private val meta: Meta) {
    val type: String =
        if (meta is Single) meta.type
        else (meta as Multi).type

    fun safe(): Multi? = if (meta is Single) null else meta as? Multi

    fun conjunctionElse(flag: Boolean): String =
        if (flag && meta is Single) meta.type
        else (meta as Multi).type
}
`)
	if len(findings) != 0 {
		t.Fatalf("meta in the else branch is a Meta that is not Single, got %d findings: %+v", len(findings), findings)
	}
}

// Pin: inside the then-branch the narrowed type still proves an impossible cast.
func TestUnsafeCast_IsNarrowingStillAppliesInThenBranch(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test

sealed class Meta
data class Single(val type: String) : Meta()
data class Multi(val type: String) : Meta()

fun pick(meta: Meta): String =
    if (meta is Single) (meta as Multi).type
    else "none"
`)
	if len(findings) != 1 {
		t.Fatalf("meta is Single in the then branch, so `as Multi` can never succeed; got %d findings: %+v", len(findings), findings)
	}
}

// Pin: two unrelated final project classes with no narrowing still report.
func TestUnsafeCast_UnrelatedFinalClassesStillReport(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test

class Apple(val n: Int)
class Brick(val n: Int)

fun convert(apple: Apple): Brick = apple as Brick
`)
	if len(findings) != 1 {
		t.Fatalf("Apple and Brick are unrelated final classes; got %d findings: %+v", len(findings), findings)
	}
}
