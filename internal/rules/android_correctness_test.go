package rules_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// DefaultLocale (CheckLines)
// ---------------------------------------------------------------------------

func TestDefaultLocale(t *testing.T) {
	t.Run("positive String.format without Locale", func(t *testing.T) {
		findings := runRuleByName(t, "DefaultLocale", `
package test
fun example() {
    val s = String.format("%d items", count)
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("positive toLowerCase without Locale", func(t *testing.T) {
		findings := runRuleByName(t, "DefaultLocale", `
package test
fun example() {
    val s = name.toLowerCase()
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("positive toUpperCase without Locale", func(t *testing.T) {
		findings := runRuleByName(t, "DefaultLocale", `
package test
fun example() {
    val s = name.toUpperCase()
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative String.format with Locale", func(t *testing.T) {
		findings := runRuleByName(t, "DefaultLocale", `
package test
fun example() {
    val s = String.format(Locale.US, "%d items", count)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative lowercase with Locale", func(t *testing.T) {
		findings := runRuleByName(t, "DefaultLocale", `
package test
fun example() {
    val s = name.lowercase(Locale.ROOT)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative Kotlin lowercase uppercase no-arg are locale invariant", func(t *testing.T) {
		findings := runRuleByName(t, "DefaultLocale", `
package test
fun example() {
    val a = name.lowercase()
    val b = name.uppercase()
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("positive Java String.format without Locale", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "DefaultLocale", `
package test;
class Formatter {
  String format(int count) {
    return String.format("%d items", count);
  }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected Java String.format finding")
		}
	})
	t.Run("positive Java case conversion without Locale", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "DefaultLocale", `
package test;
class Formatter {
  String normalize(String name) {
    return name.toLowerCase();
  }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected Java toLowerCase finding")
		}
	})
	t.Run("negative Java calls with Locale", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "DefaultLocale", `
package test;
import java.util.Locale;
class Formatter {
  String normalize(String name, int count) {
    String a = name.toUpperCase(Locale.ROOT);
    return String.format(Locale.US, "%d items", count) + a;
  }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative Java local format lookalike", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "DefaultLocale", `
package test;
class Formatter {
  static class LocalString {
    static String format(String pattern, int count) { return pattern; }
  }
  String format(int count) {
    return LocalString.format("%d items", count);
  }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for Java local lookalike, got %d", len(findings))
		}
	})
	t.Run("negative comments and strings with method names", func(t *testing.T) {
		findings := runRuleByName(t, "DefaultLocale", `
package test
fun example() {
    // name.toLowerCase()
    val a = "String.format(\"%d\", count)"
    val b = "name.toUpperCase()"
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for Kotlin text lookalikes, got %d", len(findings))
		}
	})
	t.Run("negative Java comments and strings with method names", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "DefaultLocale", `
package test;
class Formatter {
  String normalize(String name) {
    // name.toLowerCase();
    return "String.format(\"%d\", count) name.toUpperCase()";
  }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for Java text lookalikes, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// CommitPrefEdits (CheckNode - call_expression)
// ---------------------------------------------------------------------------

func TestCommitPrefEdits(t *testing.T) {
	t.Run("positive edit without commit or apply", func(t *testing.T) {
		findings := runRuleByName(t, "CommitPrefEdits", `
package test
fun save() {
    val editor = prefs.edit()
    editor.putString("key", "value")
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative edit with apply", func(t *testing.T) {
		findings := runRuleByName(t, "CommitPrefEdits", `
package test
fun save() {
    prefs.edit().putString("key", "value").apply()
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative edit with commit", func(t *testing.T) {
		findings := runRuleByName(t, "CommitPrefEdits", `
package test
fun save() {
    val editor = prefs.edit()
    editor.putString("key", "value")
    editor.commit()
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("positive unrelated apply does not finalize editor", func(t *testing.T) {
		findings := runRuleByName(t, "CommitPrefEdits", `
package test
fun save() {
    val editor = prefs.edit()
    other.apply()
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("positive Kotlin scope apply does not persist edits", func(t *testing.T) {
		findings := runRuleByName(t, "CommitPrefEdits", `
package test
fun save() {
    prefs.edit().apply {
        putString("key", "value")
    }
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative comment mentions do not create edit call", func(t *testing.T) {
		findings := runRuleByName(t, "CommitPrefEdits", `
package test
fun docsOnly() {
    // prefs.edit().putString("key", "value").commit()
    val note = "No editor is created here"
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("positive string commit mention does not finalize editor", func(t *testing.T) {
		findings := runRuleByName(t, "CommitPrefEdits", `
package test
fun save() {
    val editor = prefs.edit()
    val note = ".commit() later"
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("Java positive edit without commit or apply", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "CommitPrefEdits", `
package test;
import android.content.SharedPreferences;
class Store {
  void save(SharedPreferences prefs) {
    SharedPreferences.Editor editor = prefs.edit();
    editor.putString("key", "value");
  }
}`)
		if len(findings) == 0 {
			t.Fatal("expected Java CommitPrefEdits finding")
		}
	})
	t.Run("Java negative chained apply", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "CommitPrefEdits", `
package test;
import android.content.SharedPreferences;
class Store {
  void save(SharedPreferences prefs) {
    prefs.edit().putString("key", "value").apply();
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("Java negative local lookalike", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "CommitPrefEdits", `
package test;
class Prefs {
  Editor edit() { return new Editor(); }
}
class Editor {
  void putString(String key, String value) {}
}
class Store {
  void save(Prefs prefs) {
    Editor editor = prefs.edit();
    editor.putString("key", "value");
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for local lookalike, got %d", len(findings))
		}
	})
	t.Run("Java semantic facts suppress imported local lookalike", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithSemanticCalls(t, "CommitPrefEdits", `
package test;
import android.content.SharedPreferences;
class Prefs {
  Editor edit() { return new Editor(); }
}
class Editor {
  void putString(String key, String value) {}
}
class Store {
  void save(Prefs prefs) {
    Editor editor = prefs.edit();
    editor.putString("key", "value");
  }
}`, javaSemanticCallSpec{Callee: "edit", ReceiverType: "test.Prefs", ReturnType: "test.Editor"})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for javac-confirmed local lookalike, got %d", len(findings))
		}
	})
	t.Run("Java semantic facts suppress same simple-name lookalike", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithSemanticCalls(t, "CommitPrefEdits", `
package test;
import android.content.SharedPreferences;
class SharedPreferences {
  Editor edit() { return new Editor(); }
}
class Editor {
  void putString(String key, String value) {}
}
class Store {
  void save(SharedPreferences prefs) {
    Editor editor = prefs.edit();
    editor.putString("key", "value");
  }
}`, javaSemanticCallSpec{Callee: "edit", ReceiverType: "test.SharedPreferences", ReturnType: "test.Editor"})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for same simple-name javac-confirmed lookalike, got %d", len(findings))
		}
	})
	t.Run("Java unresolved semantic facts fall back to imported type", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithSemanticCalls(t, "CommitPrefEdits", `
package test;
import android.content.SharedPreferences;
class Store {
  void save(SharedPreferences prefs) {
    SharedPreferences.Editor editor = prefs.edit();
    editor.putString("key", "value");
  }
}`, javaSemanticCallSpec{Callee: "edit", ReceiverType: "", ReturnType: ""})
		if len(findings) == 0 {
			t.Fatal("expected Java CommitPrefEdits finding")
		}
	})
}

// Oracle resolves `.edit()` to a non-SharedPreferences callable → suppressed.
func TestCommitPrefEdits_OracleSuppressesNonSharedPreferencesEdit(t *testing.T) {
	findings := runCommitPrefEditsWithCallTarget(t, `
package test
fun save() {
    val editor = collection.edit()
    editor.add("item")
}
`, "collection.edit", "com.acme.local.Collection.edit")
	if len(findings) != 0 {
		t.Fatalf("expected oracle to suppress non-SharedPreferences edit, got %d: %v", len(findings), findings)
	}
}

// Oracle confirms android.content.SharedPreferences.edit → fires.
func TestCommitPrefEdits_OracleConfirmsSharedPreferencesEdit(t *testing.T) {
	findings := runCommitPrefEditsWithCallTarget(t, `
package test
fun save() {
    val editor = prefs.edit()
    editor.putString("key", "value")
}
`, "prefs.edit", "android.content.SharedPreferences.edit")
	if len(findings) != 1 {
		t.Fatalf("expected oracle-confirmed SharedPreferences.edit to fire, got %d: %v", len(findings), findings)
	}
}

// Pins NeedsOracleCallTargets on the rule.
func TestCommitPrefEdits_DeclaresOracleCallTargets(t *testing.T) {
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "CommitPrefEdits" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("CommitPrefEdits rule not registered")
	}
	if !rule.Needs.Has(api.NeedsOracleCallTargets) {
		t.Errorf("missing NeedsOracleCallTargets: Needs=%b", rule.Needs)
	}
}

func runCommitPrefEditsWithCallTarget(t *testing.T, code string, callText string, target string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if target != "" && strings.Contains(strings.TrimSpace(file.FlatNodeText(idx)), callText) {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = target
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == "CommitPrefEdits" {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule not found in registry")
	return nil
}

func TestCommitPrefEdits_OracleUnresolvedStaysSilent(t *testing.T) {
	findings := runCommitPrefEditsWithCallTarget(t, `
package test
fun save() {
    val editor = prefs.edit()
    editor.putString("key", "value")
}
`, "prefs.edit", "")
	if len(findings) != 0 {
		t.Fatalf("expected unresolved oracle to suppress CommitPrefEdits, got %d: %v", len(findings), findings)
	}
}

// ---------------------------------------------------------------------------
// CommitTransaction (CheckNode - call_expression)
// ---------------------------------------------------------------------------

func TestCommitTransaction(t *testing.T) {
	t.Run("positive beginTransaction without commit", func(t *testing.T) {
		findings := runRuleByName(t, "CommitTransaction", `
package test
fun showFragment() {
    supportFragmentManager.beginTransaction()
        .replace(R.id.container, fragment)
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative beginTransaction with commit", func(t *testing.T) {
		findings := runRuleByName(t, "CommitTransaction", `
package test
fun showFragment() {
    supportFragmentManager.beginTransaction()
        .replace(R.id.container, fragment)
        .commit()
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative beginTransaction assigned then committed", func(t *testing.T) {
		findings := runRuleByName(t, "CommitTransaction", `
package test
fun showFragment() {
    val tx = supportFragmentManager.beginTransaction()
    tx.replace(R.id.container, fragment)
    tx.commitAllowingStateLoss()
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("positive unrelated commit does not satisfy transaction", func(t *testing.T) {
		findings := runRuleByName(t, "CommitTransaction", `
package test
fun showFragment(other: Editor) {
    val tx = supportFragmentManager.beginTransaction()
    other.commit()
}`)
		if len(findings) == 0 {
			t.Fatal("expected finding when only unrelated receiver commits")
		}
	})
	t.Run("positive string commit mention does not satisfy transaction", func(t *testing.T) {
		findings := runRuleByName(t, "CommitTransaction", `
package test
fun showFragment() {
    val tx = supportFragmentManager.beginTransaction()
    val note = "commit() later"
}`)
		if len(findings) == 0 {
			t.Fatal("expected finding when commit appears only in a string")
		}
	})
	t.Run("Java positive beginTransaction without commit", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "CommitTransaction", `
package test;
import androidx.fragment.app.FragmentManager;
class Screen {
  void show(FragmentManager manager) {
    manager.beginTransaction().replace(1, new Object());
  }
}`)
		if len(findings) == 0 {
			t.Fatal("expected Java CommitTransaction finding")
		}
	})
	t.Run("Java negative beginTransaction with commit", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "CommitTransaction", `
package test;
import androidx.fragment.app.FragmentManager;
class Screen {
  void show(FragmentManager manager) {
    manager.beginTransaction().replace(1, new Object()).commit();
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("Java semantic facts suppress imported local lookalike", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithSemanticCalls(t, "CommitTransaction", `
package test;
import androidx.fragment.app.FragmentManager;
class Manager {
  Transaction beginTransaction() { return new Transaction(); }
}
class Transaction {
  Transaction replace(int id, Object fragment) { return this; }
}
class Screen {
  void show(Manager manager) {
    manager.beginTransaction().replace(1, new Object());
  }
}`, javaSemanticCallSpec{Callee: "beginTransaction", ReceiverType: "test.Manager", ReturnType: "test.Transaction"})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for javac-confirmed local lookalike, got %d", len(findings))
		}
	})
	t.Run("Java unresolved semantic facts fall back to imported type", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithSemanticCalls(t, "CommitTransaction", `
package test;
import androidx.fragment.app.FragmentManager;
class Screen {
  void show(FragmentManager manager) {
    manager.beginTransaction().replace(1, new Object());
  }
}`, javaSemanticCallSpec{Callee: "beginTransaction", ReceiverType: "", ReturnType: ""})
		if len(findings) == 0 {
			t.Fatal("expected Java CommitTransaction finding")
		}
	})
}

// Oracle resolves `.beginTransaction()` to a non-FragmentManager callable → suppressed.
func TestCommitTransaction_OracleSuppressesNonFragmentManager(t *testing.T) {
	findings := runCommitTransactionWithCallTarget(t, `
package test
fun show() {
    val tx = db.beginTransaction()
    tx.run()
}
`, "db.beginTransaction", "com.acme.local.Database.beginTransaction")
	if len(findings) != 0 {
		t.Fatalf("expected oracle to suppress non-FragmentManager beginTransaction, got %d: %v", len(findings), findings)
	}
}

// Oracle confirms androidx FragmentManager.beginTransaction → fires.
func TestCommitTransaction_OracleConfirmsFragmentManager(t *testing.T) {
	findings := runCommitTransactionWithCallTarget(t, `
package test
fun show() {
    val tx = supportFragmentManager.beginTransaction()
    tx.replace(R.id.container, fragment)
}
`, "supportFragmentManager.beginTransaction", "androidx.fragment.app.FragmentManager.beginTransaction")
	if len(findings) != 1 {
		t.Fatalf("expected oracle-confirmed FragmentManager.beginTransaction to fire, got %d: %v", len(findings), findings)
	}
}

// Pins NeedsOracleCallTargets on the rule.
func TestCommitTransaction_DeclaresOracleCallTargets(t *testing.T) {
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "CommitTransaction" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("CommitTransaction rule not registered")
	}
	if !rule.Needs.Has(api.NeedsOracleCallTargets) {
		t.Errorf("missing NeedsOracleCallTargets: Needs=%b", rule.Needs)
	}
}

func runCommitTransactionWithCallTarget(t *testing.T, code string, callText string, target string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if target != "" && strings.Contains(strings.TrimSpace(file.FlatNodeText(idx)), callText) {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = target
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == "CommitTransaction" {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule not found in registry")
	return nil
}

func TestCommitTransaction_OracleUnresolvedStaysSilent(t *testing.T) {
	findings := runCommitTransactionWithCallTarget(t, `
package test
fun show() {
    val tx = supportFragmentManager.beginTransaction()
    tx.replace(R.id.container, fragment)
}
`, "supportFragmentManager.beginTransaction", "")
	if len(findings) != 0 {
		t.Fatalf("expected unresolved oracle to suppress CommitTransaction, got %d: %v", len(findings), findings)
	}
}

// ---------------------------------------------------------------------------
// Assert (CheckLines)
// ---------------------------------------------------------------------------

func TestAssertRule(t *testing.T) {
	t.Run("positive assert statement", func(t *testing.T) {
		findings := runRuleByName(t, "Assert", `
package test
fun example() {
    assert(x > 0)
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative no assert", func(t *testing.T) {
		findings := runRuleByName(t, "Assert", `
package test
fun example() {
    require(x > 0)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative assert in comment", func(t *testing.T) {
		findings := runRuleByName(t, "Assert", `
package test
fun example() {
    // assert(x > 0) is bad on Android
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// CheckResult (CheckNode - call_expression)
// ---------------------------------------------------------------------------

func TestCheckResult(t *testing.T) {
	t.Run("negative consumed replace result", func(t *testing.T) {
		findings := runRuleByName(t, "CheckResult", `
package test
fun example() {
    val s = "hello"
    val result = s.replace("h", "j")
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative lambda result replace", func(t *testing.T) {
		findings := runRuleByName(t, "CheckResult", `
package test
fun example(values: List<String>) {
    values.map { value -> value.replace("a", "b") }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("positive discarded replace result", func(t *testing.T) {
		findings := runRuleByName(t, "CheckResult", `
package test
fun example() {
    "hello".replace("h", "j")
}`)
		if len(findings) == 0 {
			t.Fatal("expected finding for discarded replace result")
		}
	})
	t.Run("negative comment and string mentions do not create calls", func(t *testing.T) {
		findings := runRuleByName(t, "CheckResult", `
package test
fun example() {
    // String.format("%s", name)
    val note = "value.trim().replace(a, b)"
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative no check-result methods", func(t *testing.T) {
		findings := runRuleByName(t, "CheckResult", `
package test
fun example() {
    println("hello")
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("Java positive discarded replace result", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "CheckResult", `
package test;
class Strings {
  void example(String value) {
    value.replace("a", "b");
  }
}`)
		if len(findings) == 0 {
			t.Fatal("expected Java CheckResult finding")
		}
	})
	t.Run("Java unresolved semantic facts fall back to imported type", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithSemanticCalls(t, "CheckResult", `
package test;
import android.content.SharedPreferences;
class Settings {
  void example(SharedPreferences preferences) {
    preferences.edit();
  }
}`, javaSemanticCallSpec{Callee: "edit", ReceiverType: "", ReturnType: ""})
		if len(findings) == 0 {
			t.Fatal("expected Java CheckResult finding")
		}
	})
	t.Run("Java positive parenthesized discarded trim result", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "CheckResult", `
package test;
class Strings {
  void example(String value) {
    (value.trim());
  }
}`)
		if len(findings) == 0 {
			t.Fatal("expected Java CheckResult finding")
		}
	})
	t.Run("Java negative consumed replace result", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "CheckResult", `
package test;
class Strings {
  String example(String value) {
    return value.replace("a", "b");
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("Java negative nested argument result", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "CheckResult", `
package test;
class Strings {
  void consume(String value) {}
  void example(String value) {
    consume(value.trim());
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("Java semantic facts suppress non-String replace lookalike", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithSemanticCalls(t, "CheckResult", `
package test;
class MutableValue {
  MutableValue replace(String a, String b) { return this; }
}
class Strings {
  void example(MutableValue value) {
    value.replace("a", "b");
  }
}`, javaSemanticCallSpec{Callee: "replace", ReceiverType: "test.MutableValue", ReturnType: "test.MutableValue"})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for javac-confirmed non-String replace, got %d", len(findings))
		}
	})
}

// Oracle resolves `.replace(...)` to a non-String callable → suppressed.
func TestCheckResult_OracleSuppressesNonStringReplace(t *testing.T) {
	findings := runCheckResultWithCallTarget(t, `
package test
fun example(value: MutableValue) {
    value.replace("a", "b")
}
`, "value.replace", "com.acme.MutableValue.replace")
	if len(findings) != 0 {
		t.Fatalf("expected oracle to suppress non-String replace, got %d: %v", len(findings), findings)
	}
}

// Oracle confirms kotlin.text.StringsKt.format → fires.
func TestCheckResult_OracleConfirmsStringFormat(t *testing.T) {
	findings := runCheckResultWithCallTarget(t, `
package test
fun example() {
    String.format("hello %s", "world")
}
`, "String.format", "kotlin.text.StringsKt.format")
	if len(findings) != 1 {
		t.Fatalf("expected oracle-confirmed StringsKt.format to fire, got %d: %v", len(findings), findings)
	}
}

func TestCheckResult_DeclaresOracleCallTargets(t *testing.T) {
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "CheckResult" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("CheckResult rule not registered")
	}
	if !rule.Needs.Has(api.NeedsOracleCallTargets) {
		t.Errorf("missing NeedsOracleCallTargets: Needs=%b", rule.Needs)
	}
}

func runCheckResultWithCallTarget(t *testing.T, code string, callText string, target string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if target != "" && strings.Contains(strings.TrimSpace(file.FlatNodeText(idx)), callText) {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = target
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == "CheckResult" {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule not found in registry")
	return nil
}

func TestCheckResult_OracleUnresolvedStaysSilent(t *testing.T) {
	findings := runCheckResultWithCallTarget(t, `
package test
fun example() {
    String.format("hello %s", "world")
}
`, "String.format", "")
	if len(findings) != 0 {
		t.Fatalf("expected unresolved oracle to suppress CheckResult, got %d: %v", len(findings), findings)
	}
}

// ---------------------------------------------------------------------------
// ShiftFlags (CheckLines)
// ---------------------------------------------------------------------------

func TestShiftFlags(t *testing.T) {
	t.Run("positive flag constant without shift", func(t *testing.T) {
		findings := runRuleByName(t, "ShiftFlags", `
package test
const val MY_FLAG_A = 1
const val MY_FLAG_B = 2
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative zero one masks and combinations", func(t *testing.T) {
		findings := runRuleByName(t, "ShiftFlags", `
package test
const val MY_FLAG_NONE = 0
const val MY_FLAG_ONE = 1
const val MY_FLAG_ALL = 7
const val MY_FLAG_BOTH = MY_FLAG_A or MY_FLAG_B
const val MY_FLAG_MASK = 0xFF
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("positive hex single bit", func(t *testing.T) {
		findings := runRuleByName(t, "ShiftFlags", `
package test
const val MY_FLAG_HIGH = 0x10
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative flag constant with shl", func(t *testing.T) {
		findings := runRuleByName(t, "ShiftFlags", `
package test
const val MY_FLAG_A = 1 shl 0
const val MY_FLAG_B = 1 shl 1
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// UniqueConstants (CheckNode - annotation)
// ---------------------------------------------------------------------------

func TestUniqueConstants(t *testing.T) {
	t.Run("positive duplicate IntDef values", func(t *testing.T) {
		findings := runRuleByName(t, "UniqueConstants", `
package test
@IntDef(1, 2, 3, 2)
annotation class Mode
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("positive constants with the same value", func(t *testing.T) {
		findings := runRuleByName(t, "UniqueConstants", `
package test
const val FIRST = 5
const val SECOND = 5
@IntDef(FIRST, SECOND)
annotation class Mode
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("positive duplicate inside value array", func(t *testing.T) {
		findings := runRuleByName(t, "UniqueConstants", `
package test
const val FIRST = 1
const val SECOND = 1
@IntDef(flag = true, value = [FIRST, SECOND])
annotation class Mode
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative distinct constants", func(t *testing.T) {
		findings := runRuleByName(t, "UniqueConstants", `
package test
const val FIRST = 1
const val SECOND = 2
@IntDef(flag = true, value = [FIRST, SECOND])
annotation class Mode
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative unique IntDef values", func(t *testing.T) {
		findings := runRuleByName(t, "UniqueConstants", `
package test
@IntDef(1, 2, 3)
annotation class Mode
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// WrongThread (CheckNode - function_declaration)
// ---------------------------------------------------------------------------

func TestWrongThread(t *testing.T) {
	t.Run("positive worker calls same-file main-thread function", func(t *testing.T) {
		findings := runRuleByName(t, "WrongThread", `
package test
class Screen {
    @MainThread fun render() {}
    @WorkerThread fun load() { render() }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("positive main thread calls same-file worker function", func(t *testing.T) {
		findings := runRuleByName(t, "WrongThread", `
package test
class Screen {
    @WorkerThread fun load() {}
    @MainThread fun render() { load() }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative dispatch to the main thread", func(t *testing.T) {
		findings := runRuleByName(t, "WrongThread", `
package test
class Screen {
    @MainThread fun render() {}
    @WorkerThread fun load() { handler.post { render() } }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative invalidate on a non-view receiver", func(t *testing.T) {
		findings := runRuleByName(t, "WrongThread", `
package test
class Repo {
    @WorkerThread fun refresh() { cache.invalidate() }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative no WorkerThread annotation", func(t *testing.T) {
		findings := runRuleByName(t, "WrongThread", `
package test
class Worker {
    fun updateUI() {
        textView.setText("done")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative plain function", func(t *testing.T) {
		findings := runRuleByName(t, "WrongThread", `
package test
fun doWork() {
    println("working")
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// SQLiteString (CheckLines)
// ---------------------------------------------------------------------------

func TestSQLiteString(t *testing.T) {
	run := func(body string) []scanner.Finding {
		return runRuleByName(t, "SQLiteString", "package test\nfun f(db: SQLiteDatabase, x: String) {\n"+body+"\n}\n")
	}
	t.Run("STRING column in execSQL CREATE TABLE", func(t *testing.T) {
		findings := run(`db.execSQL("CREATE TABLE users (name STRING, age INTEGER)")`)
		if len(findings) != 1 || !strings.HasPrefix(findings[0].Message, "Using column type STRING; did you mean to use TEXT?") {
			t.Fatalf("unexpected findings: %v", findings)
		}
	})
	t.Run("ALTER TABLE, concatenation and a same-file val count", func(t *testing.T) {
		if f := run(`db.execSQL("ALTER TABLE y ADD COLUMN c STRING")`); len(f) != 1 {
			t.Fatalf("ALTER TABLE: %v", f)
		}
		if f := run(`db.execSQL("CREATE TABLE y (a STRING)" + x)`); len(f) != 1 {
			t.Fatalf("concatenation: %v", f)
		}
		findings := runRuleByName(t, "SQLiteString", "package test\nconst val DDL = \"CREATE TABLE y (a STRING)\"\nfun f(db: SQLiteDatabase) { db.execSQL(DDL) }\n")
		if len(findings) != 1 || findings[0].Line != 3 {
			t.Fatalf("val reference should report at the execSQL argument: %v", findings)
		}
	})
	t.Run("Lint ignores TEXT, other calls, lowercase SQL and lowercase string", func(t *testing.T) {
		for _, body := range []string{
			`db.execSQL("CREATE TABLE users (name TEXT, age INTEGER)")`,
			`db.rawQuery("CREATE TABLE y (a STRING)", null)`,
			`db.execSQL("create table y (a STRING)")`,
			`db.execSQL("CREATE TABLE y (a string)")`,
			`db.execSQL("Create Table y (a STRING)")`,
			`db.execSQL("  CREATE TABLE y (a STRING)")`,
			`db.execSQL("CREATE TABLE users (a string_value TEXT, b STRING_ID TEXT)")`,
		} {
			if f := run(body); len(f) != 0 {
				t.Errorf("%s: expected no findings, got %v", body, f)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Registered (CheckNode - class_declaration)
// ---------------------------------------------------------------------------

func TestRegisteredRule(t *testing.T) {
	t.Run("positive Activity subclass", func(t *testing.T) {
		findings := runRuleByName(t, "Registered", `
package test
class MainActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {}
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative abstract Activity", func(t *testing.T) {
		findings := runRuleByName(t, "Registered", `
package test
abstract class BaseActivity : AppCompatActivity() {
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative AndroidEntryPoint", func(t *testing.T) {
		findings := runRuleByName(t, "Registered", `
package test
@AndroidEntryPoint
class MainActivity : AppCompatActivity() {
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// SimpleDateFormat (CheckLines)
// ---------------------------------------------------------------------------

func TestSimpleDateFormatOtherPackage(t *testing.T) {
	t.Run("qualified call outside java.text is clean", func(t *testing.T) {
		findings := runRuleByName(t, "SimpleDateFormat", `
package test
fun format() { val sdf = foo.SimpleDateFormat("yyyy") }`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("import from another package is clean", func(t *testing.T) {
		findings := runRuleByName(t, "SimpleDateFormat", `
package test
import com.example.SimpleDateFormat
fun format() { val sdf = SimpleDateFormat("yyyy") }`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("qualified java.text call reports", func(t *testing.T) {
		findings := runRuleByName(t, "SimpleDateFormat", `
package test
fun format() { val sdf = java.text.SimpleDateFormat("yyyy") }`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

func TestSimpleDateFormatRule(t *testing.T) {
	t.Run("positive without Locale", func(t *testing.T) {
		findings := runRuleByName(t, "SimpleDateFormat", `
package test
fun format() {
    val sdf = SimpleDateFormat("yyyy-MM-dd")
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative with Locale", func(t *testing.T) {
		findings := runRuleByName(t, "SimpleDateFormat", `
package test
fun format() {
    val sdf = SimpleDateFormat("yyyy-MM-dd", Locale.US)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("positive Java without Locale", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "SimpleDateFormat", `
package test;
import java.text.SimpleDateFormat;
class Dates {
  Object format() {
    return new SimpleDateFormat("yyyy-MM-dd");
  }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected Java SimpleDateFormat finding")
		}
	})
	t.Run("negative Java with Locale", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "SimpleDateFormat", `
package test;
import java.text.SimpleDateFormat;
import java.util.Locale;
class Dates {
  Object format() {
    return new SimpleDateFormat("yyyy-MM-dd", Locale.US);
  }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative Java local lookalike", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "SimpleDateFormat", `
package test;
class SimpleDateFormat {
  SimpleDateFormat(String pattern) {}
}
class Dates {
  Object format() {
    return new SimpleDateFormat("yyyy-MM-dd");
  }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for Java local lookalike, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// SetTextI18n (CheckLines)
// ---------------------------------------------------------------------------

func TestSetTextI18n(t *testing.T) {
	t.Run("positive hardcoded setText", func(t *testing.T) {
		findings := runRuleByName(t, "SetTextI18n", `
package test
fun update() {
    textView.setText("Hello World")
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative setText with resource", func(t *testing.T) {
		findings := runRuleByName(t, "SetTextI18n", `
package test
fun update() {
    textView.setText(R.string.hello)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative comments and strings with setText literal", func(t *testing.T) {
		findings := runRuleByName(t, "SetTextI18n", `
package test
fun update() {
    // textView.setText("Hello World")
    val code = "textView.setText(\"Hello World\")"
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for text lookalikes, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StopShip (CheckLines)
// ---------------------------------------------------------------------------

func TestStopShipIgnoresStringLiterals(t *testing.T) {
	findings := runRuleByName(t, "StopShip", `
package test
val label = "STOPSHIP in a string"
val tagged = "x" // STOPSHIP real marker
/*
 * STOPSHIP in a block
 */
`)
	if len(findings) != 2 {
		t.Fatalf("expected the two comment markers only, got %d", len(findings))
	}
}

func TestStopShip(t *testing.T) {
	t.Run("positive STOPSHIP comment", func(t *testing.T) {
		findings := runRuleByName(t, "StopShip", `
package test
// STOPSHIP: remove before release
fun debug() {}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative no STOPSHIP", func(t *testing.T) {
		findings := runRuleByName(t, "StopShip", `
package test
// TODO: improve this later
fun work() {}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// WrongCall (CheckLines)
// ---------------------------------------------------------------------------

func TestWrongCallInDifferentOverride(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "WrongCall", `
package test
import android.view.View
class CustomView : View() {
    private val child: View = TODO()
    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        child.onMeasure(1, 2)
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestWrongCall(t *testing.T) {
	t.Run("positive direct onDraw call inside View subclass", func(t *testing.T) {
		// Receiver-type proof: enclosing class extends View, so child.onDraw()
		// dispatches to a View method and should fire.
		findings := runRuleByNameWithResolver(t, "WrongCall", `
package test
import android.view.View
class CustomView : View() {
    private val child: View = TODO()
    fun forceRedraw(canvas: android.graphics.Canvas) {
        child.onDraw(canvas)
    }
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative super.onDraw call", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "WrongCall", `
package test
override fun onDraw(canvas: Canvas) {
    super.onDraw(canvas)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative bare onDraw call with no receiver evidence", func(t *testing.T) {
		// No enclosing class, no resolver receiver evidence — must not fire.
		findings := runRuleByNameWithResolver(t, "WrongCall", `
package test
fun refresh() {
    view.onDraw(canvas)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative onDraw on non-View receiver root", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "WrongCall", `
package test
class Renderer {
    fun draw(canvas: android.graphics.Canvas) {
        val h = Helper()
        h.onDraw(canvas)
    }
}
class Helper { fun onDraw(c: android.graphics.Canvas) {} }`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative onDraw on NotificationCompat builder chain", func(t *testing.T) {
		// Receiver chain rooted at a known non-View symbol — must not fire.
		findings := runRuleByNameWithResolver(t, "WrongCall", `
package test
class Renderer {
    fun build(builder: Any) {
        NotificationCompat.Builder(builder, "ch").onDraw(builder)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// ScrollViewCount (Kotlin source heuristic — primary signal is the XML rule)
// ---------------------------------------------------------------------------

func TestScrollViewCount(t *testing.T) {
	t.Run("positive ScrollView apply with multiple addView", func(t *testing.T) {
		findings := runRuleByName(t, "ScrollViewCount", `
package test
fun build(context: android.content.Context) {
    ScrollView(context).apply {
        addView(View(context))
        addView(View(context))
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("positive HorizontalScrollView apply with multiple addView", func(t *testing.T) {
		findings := runRuleByName(t, "ScrollViewCount", `
package test
fun build(context: android.content.Context) {
    HorizontalScrollView(context).apply {
        addView(View(context))
        addView(View(context))
        addView(View(context))
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative single addView", func(t *testing.T) {
		findings := runRuleByName(t, "ScrollViewCount", `
package test
fun build(context: android.content.Context) {
    ScrollView(context).apply {
        addView(LinearLayout(context).apply {
            addView(View(context))
            addView(View(context))
        })
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative non-scroll receiver", func(t *testing.T) {
		findings := runRuleByName(t, "ScrollViewCount", `
package test
fun build(context: android.content.Context) {
    LinearLayout(context).apply {
        addView(View(context))
        addView(View(context))
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative empty apply", func(t *testing.T) {
		findings := runRuleByName(t, "ScrollViewCount", `
package test
fun build(context: android.content.Context) {
    ScrollView(context).apply {}
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative comments and strings with ScrollView addView", func(t *testing.T) {
		findings := runRuleByName(t, "ScrollViewCount", `
package test
fun build(context: android.content.Context) {
    // ScrollView(context).apply { addView(View(context)); addView(View(context)) }
    val code = "ScrollView(context).apply { addView(View(context)); addView(View(context)) }"
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for text lookalikes, got %d", len(findings))
		}
	})
}

func TestCommitPrefEdits_EscapingAndKtxEditors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"ktx trailing lambda", "fun f(p: SharedPreferences) { p.edit { putString(\"a\", \"b\") } }", 0},
		{"expression body returns editor", "fun f(p: SharedPreferences) = p.edit().putString(\"a\", \"b\")", 0},
		{"returned directly", "fun f(p: SharedPreferences): SharedPreferences.Editor { return p.edit() }", 0},
		{"local returned later", "fun f(p: SharedPreferences): SharedPreferences.Editor { val ed = p.edit(); return ed }", 0},
		{"passed as argument", "fun f(p: SharedPreferences) { use(p.edit()) }", 0},
		{"local passed as argument", "fun f(p: SharedPreferences) { val ed = p.edit(); use(ed) }", 0},
		{"chain from local ends in apply", "fun f(p: SharedPreferences) { val ed = p.edit(); ed.putInt(\"a\", 1).apply() }", 0},
		{"edited but never finalized", "fun f(p: SharedPreferences) { p.edit().putInt(\"a\", 1) }", 1},
		{"local never finalized", "fun f(p: SharedPreferences) { val ed = p.edit(); ed.putInt(\"a\", 1) }", 1},
		{"also it apply", "fun f(p: SharedPreferences) { p.edit().also { it.apply() } }", 0},
		{"run bare commit", "fun f(p: SharedPreferences) { p.edit().run { putInt(\"a\", 1); commit() } }", 0},
		{"let named parameter apply", "fun f(p: SharedPreferences) { p.edit().let { e -> e.apply() } }", 0},
		{"nested lambda apply does not finalize", "fun f(p: SharedPreferences, xs: List<Int>) { p.edit().apply { xs.forEach { apply() } } }", 1},
		{"also without finalize", "fun f(p: SharedPreferences) { p.edit().also { it.putInt(\"a\", 1) } }", 1},
		{"local alias finalized", "fun f(p: SharedPreferences) { val ed = p.edit(); val e2 = ed; e2.apply() }", 0},
		{"local apply lambda finalizes", "fun f(p: SharedPreferences) { val ed = p.edit(); ed.run { apply() } }", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "CommitPrefEdits", "package test\nimport android.content.SharedPreferences\nfun use(e: SharedPreferences.Editor) {}\n"+tc.body+"\n")
			if len(findings) != tc.want {
				t.Fatalf("expected %d findings, got %d: %v", tc.want, len(findings), findings)
			}
		})
	}
}

func TestCommitTransaction_ReturnedTransactionIsNotReported(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"expression body returns it", "fun f(fm: FragmentManager) = fm.beginTransaction()", 0},
		{"returned local", "fun f(fm: FragmentManager): FragmentTransaction { val tx = fm.beginTransaction(); return tx }", 0},
		{"never committed", "fun f(fm: FragmentManager) { fm.beginTransaction().add(1, g()) }", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "CommitTransaction", "package test\nimport androidx.fragment.app.FragmentManager\nimport androidx.fragment.app.FragmentTransaction\n"+tc.body+"\n")
			if len(findings) != tc.want {
				t.Fatalf("expected %d findings, got %d", tc.want, len(findings))
			}
		})
	}
}

func TestAssertRule_OnlyStdlibAssert(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"bare assert", "package test\nfun f() { assert(1 == 1) }\n", 1},
		{"kotlin qualified", "package test\nfun f() { kotlin.assert(true) }\n", 1},
		{"member assert", "package test\nclass Foo { fun assert(b: Boolean) {} }\nfun f(x: Foo) { x.assert(true) }\n", 0},
		{"file local assert function", "package test\nfun assert(b: Boolean) {}\nfun f() { assert(true) }\n", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(runRuleByName(t, "Assert", tc.src)); got != tc.want {
				t.Fatalf("expected %d findings, got %d", tc.want, got)
			}
		})
	}
}

func TestCheckResult_KtxEditLambdaIsNotReported(t *testing.T) {
	src := "package test\nimport android.content.SharedPreferences\nfun f(p: SharedPreferences) {\n  p.edit { putInt(\"a\", 1) }\n}\n"
	if got := runRuleByName(t, "CheckResult", src); len(got) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(got))
	}
}

func TestCommitTransactionJava_JudgesEachTransactionAlone(t *testing.T) {
	code := `package p;

import android.app.FragmentManager;
import android.app.FragmentTransaction;

class B {
    void tx(FragmentManager fm) {
        FragmentTransaction t = fm.beginTransaction();
        t.add(0, null);
        FragmentTransaction u = fm.beginTransaction();
        u.commit();
        fm.beginTransaction().add(0, null);
        fm.beginTransaction().add(0, null).commit();
    }
    FragmentTransaction handedBack(FragmentManager fm) {
        FragmentTransaction v = fm.beginTransaction();
        return v;
    }
    FragmentTransaction chainReturned(FragmentManager fm) {
        return fm.beginTransaction();
    }
}
`
	findings := runRuleByNameOnJava(t, "CommitTransaction", code)
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
		if f.Message != "This transaction should be completed with a `commit()` call" {
			t.Errorf("unexpected message %q", f.Message)
		}
	}
	if len(lines) != 2 || lines[0] != 8 || lines[1] != 12 {
		t.Fatalf("expected findings on lines 8 and 12, got %v", lines)
	}
}

func TestClickableViewAccessibilitySource(t *testing.T) {
	messages := func(findings []scanner.Finding) []string {
		var out []string
		for _, f := range findings {
			out = append(out, f.Message)
		}
		sort.Strings(out)
		return out
	}
	t.Run("Kotlin custom views", func(t *testing.T) {
		got := messages(runRuleByName(t, "ClickableViewAccessibility", `
package p
import android.content.Context
import android.view.MotionEvent
import android.view.View
import android.widget.ImageView

class NoClick(c: Context) : View(c) {
    override fun onTouchEvent(e: MotionEvent): Boolean = true
}
class Forgets(c: Context) : ImageView(c) {
    override fun onTouchEvent(e: MotionEvent): Boolean = true
    override fun performClick(): Boolean = super.performClick()
}
class Good(c: Context) : View(c) {
    override fun onTouchEvent(e: MotionEvent): Boolean {
        performClick()
        return true
    }
    override fun performClick(): Boolean = super.performClick()
}
class NotAView {
    fun onTouchEvent(e: MotionEvent): Boolean = true
}
class NoSuper(c: Context) : View(c) {
    override fun onTouchEvent(e: MotionEvent): Boolean {
        performClick()
        return true
    }
    override fun performClick(): Boolean {
        return true
    }
}
class CallsSuperOnTouch(c: Context) : View(c) {
    override fun onTouchEvent(e: MotionEvent): Boolean = super.onTouchEvent(e)
    override fun performClick(): Boolean = super.performClick()
}
fun attach(v: View) {
    v.setOnTouchListener { view, e -> true }
    v.setOnTouchListener { view, e -> view.performClick(); true }
}
`))
		want := []string{
			"Custom view `NoClick` overrides `onTouchEvent` but not `performClick`",
			"`Forgets#onTouchEvent` should call `Forgets#performClick` when a click is detected",
			"`onTouch` lambda should call `View#performClick` when a click is detected",
			"`NoSuper#performClick` should call `super#performClick`",
			"`CallsSuperOnTouch#onTouchEvent` should call `CallsSuperOnTouch#performClick` when a click is detected",
		}
		sort.Strings(want)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("Java custom views and listeners", func(t *testing.T) {
		got := messages(runRuleByNameOnJava(t, "ClickableViewAccessibility", `package p;
import android.content.Context;
import android.view.MotionEvent;
import android.view.View;

class NoClick extends View {
    NoClick(Context c) { super(c); }
    @Override public boolean onTouchEvent(MotionEvent e) { return true; }
}
class Good extends View {
    Good(Context c) { super(c); }
    @Override public boolean onTouchEvent(MotionEvent e) { performClick(); return true; }
    @Override public boolean performClick() { return super.performClick(); }
}
class NoSuper extends View {
    NoSuper(Context c) { super(c); }
    @Override public boolean performClick() { return true; }
}
class Holder {
    void a(View v) {
        v.setOnTouchListener(new View.OnTouchListener() {
            @Override public boolean onTouch(View view, MotionEvent e) { return true; }
        });
        v.setOnTouchListener((view, e) -> { view.performClick(); return true; });
    }
}
`))
		want := []string{
			"Custom view `NoClick` overrides `onTouchEvent` but not `performClick`",
			"`onTouch` should call `View#performClick` when a click is detected",
			"`NoSuper#performClick` should call `super#performClick`",
		}
		sort.Strings(want)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestCutPasteIdSource(t *testing.T) {
	const msg = "The id `R.id.one` has already been looked up in this method; possible cut & paste error?"
	t.Run("Java", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "CutPasteId", `package p;
import android.app.Activity;
import android.view.View;
class A extends Activity {
    void a() {
        View x = findViewById(R.id.one);
        View y = findViewById(R.id.one);
        View z = findViewById(R.id.two);
        View w = findViewById(R.id.six);
        if (w == null) { w = findViewById(R.id.six); }
    }
    void b() {
        View q = findViewById(R.id.one);
    }
}
`)
		if len(findings) != 1 || findings[0].Line != 7 || findings[0].Message != msg {
			t.Fatalf("unexpected findings %+v", findings)
		}
	})
	t.Run("Kotlin", func(t *testing.T) {
		findings := runRuleByName(t, "CutPasteId", `
package p
import android.app.Activity
import android.view.View
class A : Activity() {
    fun a() {
        val x = findViewById<View>(R.id.one)
        val y = findViewById<View>(R.id.one)
        val z = findViewById<View>(R.id.two)
    }
    fun b() {
        val q = findViewById<View>(R.id.one)
    }
}
`)
		if len(findings) != 1 || findings[0].Line != 8 || findings[0].Message != msg {
			t.Fatalf("unexpected findings %+v", findings)
		}
	})
}
