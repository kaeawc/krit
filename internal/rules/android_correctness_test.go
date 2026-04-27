package rules_test

import (
	"testing"
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
	t.Run("positive direct onDraw call", func(t *testing.T) {
		findings := runRuleByName(t, "WrongCall", `
package test
fun refresh() {
    view.onDraw(canvas)
}`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("negative super.onDraw call", func(t *testing.T) {
		findings := runRuleByName(t, "WrongCall", `
package test
override fun onDraw(canvas: Canvas) {
    super.onDraw(canvas)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// Stub rules - verify they exist and don't crash
// ---------------------------------------------------------------------------

func TestScrollViewCount_Stub(t *testing.T) {
	findings := runRuleByName(t, "ScrollViewCount", `
package test
fun example() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings from stub rule, got %d", len(findings))
	}
}
