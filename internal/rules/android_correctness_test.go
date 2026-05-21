package rules_test

import (
	"fmt"
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

// Pins NeedsOracleCallTargets + OracleCallTargets filter on the rule.
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
	if rule.OracleCallTargets == nil || len(rule.OracleCallTargets.CalleeNames) == 0 {
		t.Errorf("OracleCallTargets must list `edit`, got %+v", rule.OracleCallTargets)
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
		if strings.Contains(strings.TrimSpace(file.FlatNodeText(idx)), callText) {
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
	t.Run("positive STRING in CREATE TABLE", func(t *testing.T) {
		findings := runRuleByName(t, "SQLiteString", `
package test
val sql = "CREATE TABLE users (name STRING, age INTEGER)"
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative TEXT in CREATE TABLE", func(t *testing.T) {
		findings := runRuleByName(t, "SQLiteString", `
package test
val sql = "CREATE TABLE users (name TEXT, age INTEGER)"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
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
// NestedScrolling (CheckLines)
// ---------------------------------------------------------------------------

func TestNestedScrollingCorrectness(t *testing.T) {
	t.Run("positive nested scroll containers", func(t *testing.T) {
		findings := runRuleByName(t, "NestedScrolling", `package test
val content = ScrollView {
    LazyColumn {
        item { }
    }
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative single scroll container", func(t *testing.T) {
		findings := runRuleByName(t, "NestedScrolling", `package test
val content = LazyColumn {
    item { }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative comments and strings with nested scroll names", func(t *testing.T) {
		findings := runRuleByName(t, "NestedScrolling", `package test
fun describe() {
    // ScrollView { LazyColumn { } }
    val text = "ScrollView { LazyColumn { item {} } }"
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for text lookalikes, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// SimpleDateFormat (CheckLines)
// ---------------------------------------------------------------------------

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
