package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCursorLoopPositiveFixture(t *testing.T) {
	path := filepath.Join(fixtureRoot(t), "positive", "resource-cost", "CursorLoopWithColumnIndexInLoop.kt")
	code, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", string(code))
	if len(findings) != 1 || findings[0].Line != 8 {
		t.Fatalf("expected positive fixture lookup at line 8, got %v", findings)
	}
}

func TestCursorLoopNegativeFixtureHasNoUnnecessaryApply(t *testing.T) {
	path := filepath.Join(fixtureRoot(t), "negative", "resource-cost", "CursorLoopWithColumnIndexInLoop.kt")
	code, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range []string{"CursorLoopWithColumnIndexInLoop", "UnnecessaryApply"} {
		if findings := runRuleByName(t, rule, string(code)); len(findings) != 0 {
			t.Fatalf("negative fixture unexpectedly reports %s: %v", rule, findings)
		}
	}
}

func TestCursorLoopReceivers(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"query local", `val items: Cursor = db.rawQuery("SELECT name FROM items WHERE order_id = ?", arrayOf(orders.getString(0)))
val nameIdx = items.getColumnIndex("name")
while (items.moveToNext()) println(items.getString(nameIdx))
items.close()`},
		{"use parameter", `db.query().use { c -> c.getColumnIndex("name") }`},
		{"use implicit parameter", `db.query().use { it.getColumnIndex("name") }`},
		{"with local", `val localCursor = db.query(); with(localCursor) { getColumnIndex("x") }`},
		{"unknown receiver", `unknown.getColumnIndex("x")`},
		{"later receiver", `later.getColumnIndex("x")`},
		{"destructured local", `val (name, child) = pair; child.getColumnIndex("x")`},
		{"destructured for", `for ((name, child) in children) child.getColumnIndex("x")`},
		{"destructured lambda", `children.forEach { (name, child) -> child.getColumnIndex("x") }`},
		{"named function", `fun idx(child: Cursor) = child.getColumnIndex("x")`},
		{"catch", `try { work() } catch (child: Exception) { child.getColumnIndex("x") }`},
		{"shadow anywhere", `orders.getColumnIndex("x"); run { orders -> println(orders) }`},
		{"destructured shadow", `val (name, orders) = pair; orders.getColumnIndex("x")`},
		{"for shadow", `for ((name, orders) in children) orders.getColumnIndex("x")`},
		{"lambda shadow", `children.forEach { (name, orders) -> orders.getColumnIndex("x") }`},
		{"function shadow", `fun idx(orders: Cursor) = orders.getColumnIndex("x")`},
		{"catch shadow", `try { work() } catch (orders: Exception) { orders.getColumnIndex("x") }`},
		{"complex destructured receiver", `val (name, child) = pair; child.identity().getColumnIndex("x")`},
		{"complex function receiver", `fun idx(child: Cursor) = child.identity().getColumnIndex("x")`},
		{"receiver expression", `val localCursor = db.query(); localCursor.also { println(it) }.getColumnIndex("x")`},
		{"empty parameters", `run { -> orders.getColumnIndex(it) }`},
		{"inline query", `db.rawQuery("items", null).getColumnIndex("x")`},
		{"inline with", `with(db.rawQuery("items", null)) { getColumnIndex("x") }`},
		{"inline apply", `db.rawQuery("items", null).apply { getColumnIndex("x") }`},
		{"inline run", `db.rawQuery("items", null).run { getColumnIndex("x") }`},
		{"standalone receiver lambda", `orders.apply { val lookup: Cursor.() -> Int = { getColumnIndex("x") }; others.forEach { it.lookup() } }`},
		{"labelled this", `db.rawQuery("items", null).apply items@{ orders.apply { this@items.getColumnIndex("x") } }`},
		{"setter shadow", `object { var c: Cursor? = null; set(orders) { field = orders; orders?.getColumnIndex("x") } }`},
		{"parenthesized", `(orders).getColumnIndex("x")`},
		{"cast", `(orders as Cursor).getColumnIndex("x")`},
		{"navigation", `orders.child.getColumnIndex("x")`},
		{"call chain", `orders.identity().getColumnIndex("x")`},
		{"reassigned", `orders = query(); orders.getColumnIndex("x")`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			code := "fun load(db: SQLiteDatabase, orders: Cursor) { while (orders.moveToNext()) { " + test.body + " } }"
			if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 0 {
				t.Fatalf("expected no hoistable lookup, got %v", findings)
			}
		})
	}
}

func TestCursorLoopFirstLookupOrder(t *testing.T) {
	for _, body := range []string{
		`cursor.getString(cursor.getColumnIndex("first"))
    .plus(cursor.getString(cursor.getColumnIndex("last")))`,
		`cursor.getString(cursor.getColumnIndex("first"))
    .also { println(cursor.getColumnIndex("last")) }`,
		`with(cursor) { getColumnIndex("ignored") }; cursor.getColumnIndex("first")`,
		`val local = cursor; local.getColumnIndex("bound"); cursor.getColumnIndex("first")`,
	} {
		code := "fun read(cursor: Cursor) { while (cursor.moveToNext()) { " + body + " } }"
		findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code)
		if len(findings) != 1 {
			t.Fatalf("expected one lookup, got %v", findings)
		}
		prefix := code[:strings.Index(code, `getColumnIndex("first")`)]
		column := len(prefix) - strings.LastIndex(prefix, "\n")
		if findings[0].Line != strings.Count(prefix, "\n")+1 || findings[0].Col != column {
			t.Fatalf("wrong first lookup: %v", findings)
		}
	}
}

func TestCursorLoopCallBeforeReceiver(t *testing.T) {
	code := `fun read(cursor: Cursor) {
    while (cursor.moveToNext()) {
        cursor.identity(cursor.getColumnIndex("inner"))
            .getColumnIndex("outer")
    }
}`
	findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code)
	if len(findings) != 1 || findings[0].Line != 3 {
		t.Fatalf("expected simple receiver lookup within the ignored chain, got %v", findings)
	}
}

func TestCursorLoopJavaLocalReceiver(t *testing.T) {
	code := `import android.database.Cursor;
class Reader {
  void read(Cursor orders) {
    while (orders.moveToNext()) {
      Cursor items = query();
      int index = items.getColumnIndex("name");
      items.close();
    }
  }
}`
	if findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 0 {
		t.Fatalf("expected no hoistable local receiver, got %v", findings)
	}
}

func TestCursorLoopJavaBindingConstructs(t *testing.T) {
	for _, body := range []string{
		`try (Cursor items = db.rawQuery("SELECT name FROM items", null)) { int index = items.getColumnIndex("name"); }`,
		`for (Cursor child : children) child.getColumnIndex("name");`,
		`children.forEach(child -> child.getColumnIndex("name"));`,
		`children.forEach((child) -> child.getColumnIndex("name"));`,
		`children.forEach((Cursor child) -> child.getColumnIndex("name"));`,
		`try { work(); } catch (Exception child) { child.getColumnIndex("name"); }`,
		`unknown.getColumnIndex("name");`,
		`try (Cursor orders = db.rawQuery("SELECT name FROM items", null)) { orders.getColumnIndex("name"); }`,
		`for (Cursor orders : children) orders.getColumnIndex("name");`,
		`children.forEach(orders -> orders.getColumnIndex("name"));`,
		`try (Cursor child = query()) { child.identity().getColumnIndex("name"); }`,
		`for (Cursor child : children) child.identity().getColumnIndex("name");`,
		`children.forEach(child -> child.identity().getColumnIndex("name"));`,
		`new Object() { void read() { getColumnIndex("name"); } };`,
		`db.rawQuery("items", null).getColumnIndex("name");`,
		`(orders).getColumnIndex("name");`,
		`((Cursor) orders).getColumnIndex("name");`,
		`orders.child.getColumnIndex("name");`,
		`this.orders.getColumnIndex("name");`,
		`orders = query(); orders.getColumnIndex("name");`,
	} {
		t.Run(body, func(t *testing.T) {
			code := `class Reader { void read(SQLiteDatabase db, Cursor orders) { while (orders.moveToNext()) { ` + body + ` } } }`
			if findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 0 {
				t.Fatalf("expected silence for unhoistable binding, got %v", findings)
			}
		})
	}
}

func TestCursorLoopProvenOuterReceivers(t *testing.T) {
	for _, body := range []string{
		`with(cursor) { getColumnIndex("x") }`,
		`cursor.apply { getColumnIndex("x") }`,
		`cursor.run { getColumnIndex("x") }`,
		`cursor.apply { this.getColumnIndex("x") }`,
		`cursor.apply { run { getColumnIndex("x") } }`,
		`cursor.use { getColumnIndex("x") }`,
	} {
		t.Run(body, func(t *testing.T) {
			code := `class Reader { fun Cursor.read(cursor: Cursor) { while (cursor.moveToNext()) { ` + body + ` } } }`
			if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 0 {
				t.Fatalf("expected silence for implicit or this receiver, got %v", findings)
			}
		})
	}
	for _, code := range []string{
		`fun read(cursor: Cursor) { val outer = cursor; while (cursor.moveToNext()) { outer.getColumnIndex("x") } }`,
		`fun read(cursor: Cursor) { var outer = cursor; if (ready) { while (cursor.moveToNext()) { outer.getColumnIndex("x") } } }`,
		`fun read(cursor: Cursor) { run { val outer = cursor; while (outer.moveToNext()) { outer.getColumnIndex("x") } } }`,
	} {
		if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 1 {
			t.Fatalf("expected proven outer binding, got %v for %s", findings, code)
		}
	}
	for _, code := range []string{
		`class Reader { void read(Cursor cursor) { while (cursor.moveToNext()) { cursor.getColumnIndex("x"); } } }`,
		`class Reader { void read(Cursor cursor) { Cursor outer = cursor; while (cursor.moveToNext()) { outer.getColumnIndex("x"); } } }`,
	} {
		if findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 1 {
			t.Fatalf("expected proven Java outer binding, got %v for %s", findings, code)
		}
	}
}

func TestCursorLoopConservativeBindings(t *testing.T) {
	for _, code := range []string{
		`class Reader(val outer: Cursor) { fun read(cursor: Cursor) { while (cursor.moveToNext()) outer.getColumnIndex("x") } }`,
		`class Reader { val outer: Cursor = query(); fun read(cursor: Cursor) { while (cursor.moveToNext()) outer.getColumnIndex("x") } }`,
		`class Reader(val pages: List<Cursor>) { var page = 0; val current: Cursor get() = pages[page]; fun read(cursor: Cursor) { while (cursor.moveToNext()) { page = cursor.getInt(0); current.getColumnIndex("x") } } }`,
		`class Reader(val cursor: Cursor) { val outer: Cursor by lazy { cursor }; fun read() { while (cursor.moveToNext()) outer.getColumnIndex("x") } }`,
		`val shared: Cursor = query(); fun read(cursor: Cursor) { while (cursor.moveToNext()) shared.getColumnIndex("x") }`,
		`fun read(cursor: Cursor) { val row: Cursor by lazy { cursor }; while (cursor.moveToNext()) row.getColumnIndex("x") }`,
		`fun read(cursor: Cursor) { while (cursor.moveToNext()) run { cursor.getColumnIndex("x") } }`,
		`fun read(cursor: Cursor) { while (cursor.moveToNext()) with(query()) { cursor.getColumnIndex("x") } }`,
		`fun read(cursor: Cursor) { while (cursor.moveToNext()) { fun lookup() = cursor.getColumnIndex("x"); lookup() } }`,
		`fun read(cursor: Cursor) { while (cursor.moveToNext()) { object { init { cursor.getColumnIndex("x") } } } }`,
		`fun read(cursor: Cursor) { val outer = cursor; fun nested() { while (cursor.moveToNext()) outer.getColumnIndex("x") } }`,
		`fun read(cursor: Cursor) { val outer = cursor; run { while (cursor.moveToNext()) outer.getColumnIndex("x") } }`,
		`class Row(val items: Cursor); class Reader(val items: Cursor) { fun read(orders: Cursor, rows: List<Row>) { while (orders.moveToNext()) with(rows[orders.position]) { items.getColumnIndex("x") } } }`,
	} {
		if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 0 {
			t.Fatalf("expected conservative miss, got %v for %s", findings, code)
		}
	}
	for _, code := range []string{
		`class Reader { Cursor outer; void read(Cursor cursor) { while (cursor.moveToNext()) { outer.getColumnIndex("x"); } } }`,
		`class Reader { Reader(Cursor cursor) { while (cursor.moveToNext()) cursor.getColumnIndex("x"); } }`,
		`class Reader { Reader(Cursor cursor) { while (cursor.moveToNext()) { cursor.getColumnIndex("x"); } } }`,
		`class Reader { void read(List<Cursor> cursors) { cursors.forEach(cursor -> { while (cursor.moveToNext()) cursor.getColumnIndex("x"); }); } }`,
		`class Reader { void read(List<Cursor> cursors) { cursors.forEach(cursor -> { while (cursor.moveToNext()) { cursor.getColumnIndex("x"); } }); } }`,
		`class Reader { void read(Cursor cursor) { while (cursor.moveToNext()) { Runnable task = () -> cursor.getColumnIndex("x"); } } }`,
		`class Reader { void read(Cursor cursor) { while (cursor.moveToNext()) { Object task = new Object() { int lookup() { return cursor.getColumnIndex("x"); } }; } } }`,
		`class Reader { void read(Cursor cursor) { Cursor outer = cursor; Runnable task = () -> { while (cursor.moveToNext()) { outer.getColumnIndex("x"); } }; } }`,
	} {
		if findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 0 {
			t.Fatalf("expected conservative Java miss, got %v for %s", findings, code)
		}
	}
}

func TestCursorLoopShadowedParameter(t *testing.T) {
	for _, body := range []string{
		`while (cursor.moveToNext()) { if (next instanceof Box(Cursor rows)) { rows.getColumnIndex("x"); } }`,
		`while (cursor.moveToNext()) { switch (next) { case Box(Cursor rows) -> rows.getColumnIndex("x"); } }`,
		`if (next instanceof Box(Cursor rows)) { while (cursor.moveToNext()) { rows.getColumnIndex("x"); } }`,
		`switch (next) { case Box(Cursor rows) -> { while (cursor.moveToNext()) { rows.getColumnIndex("x"); } } }`,
		`if (next instanceof Cursor rows) { while (cursor.moveToNext()) { rows.getColumnIndex("x"); } }`,
		`for (Cursor rows : cursors) { while (cursor.moveToNext()) { rows.getColumnIndex("x"); } }`,
		`for (Cursor rows = query(); rows.moveToNext(); ) { while (cursor.moveToNext()) { rows.getColumnIndex("x"); } }`,
		`try (Cursor rows = query()) { while (cursor.moveToNext()) { rows.getColumnIndex("x"); } }`,
		`try { work(); } catch (Exception rows) { while (cursor.moveToNext()) { rows.getColumnIndex("x"); } }`,
	} {
		code := `record Box(Cursor rows) {} class Reader { void read(Cursor cursor, Cursor rows, Object next) { ` + body + ` } }`
		if findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 0 {
			t.Fatalf("expected shadowed parameter silence, got %v for %s", findings, code)
		}
	}
	for _, body := range []string{
		`for (rows in cursors) { while (cursor.moveToNext()) rows.getColumnIndex("x") }`,
		`for ((name, rows) in children) { while (cursor.moveToNext()) rows.getColumnIndex("x") }`,
		`try { work() } catch (rows: Exception) { while (cursor.moveToNext()) rows.getColumnIndex("x") }`,
		`run { rows -> while (cursor.moveToNext()) rows.getColumnIndex("x") }`,
		`when (val rows = cursor) { else -> while (cursor.moveToNext()) rows.getColumnIndex("x") }`,
		`if (ready) { val (name, rows) = pair; while (cursor.moveToNext()) rows.getColumnIndex("x") }`,
	} {
		code := `fun read(cursor: Cursor, rows: Cursor) { ` + body + ` }`
		want := 0
		if strings.Contains(body, "val (name, rows)") || strings.Contains(body, "run { rows ->") {
			want = 1
		}
		if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != want {
			t.Fatalf("expected %d for nearest binding, got %v for %s", want, findings, code)
		}
	}
}

func TestCursorLoopUnknownOuterBinding(t *testing.T) {
	for _, code := range []string{
		`fun Cursor.read() { while (moveToNext()) { with(this) { this.getColumnIndex("x") } } }`,
		`fun read(cursor: Cursor) { while (cursor.moveToNext()) { later.getColumnIndex("x") }; val later = cursor }`,
		`fun read(cursor: Cursor) { run { val unrelated = cursor }; while (cursor.moveToNext()) { unrelated.getColumnIndex("x") } }`,
		`fun read(cursor: Cursor) { val callback = { unrelated: Cursor -> unrelated }; while (cursor.moveToNext()) { unrelated.getColumnIndex("x") } }`,
		`fun read(callback: (unrelated: Cursor) -> Unit, cursor: Cursor) { while (cursor.moveToNext()) { unrelated.getColumnIndex("x") } }`,
		`fun Cursor.read() { while (moveToNext()) { customScope { getColumnIndex("x") } } }`,
		`fun read(cursor: Cursor) { val uninitialized = run { while (cursor.moveToNext()) { uninitialized.getColumnIndex("x") }; cursor } }`,
		`class Reader(unrelated: Cursor) { fun read(cursor: Cursor) { while (cursor.moveToNext()) { unrelated.getColumnIndex("x") } } }`,
		`fun read(cursor: Cursor) { run { -> while (cursor.moveToNext()) { it.getColumnIndex("x") } } }`,
		`fun read(cursor: Cursor) { var outer = cursor; outer = query(); while (cursor.moveToNext()) { outer.getColumnIndex("x") } }`,
		`class Reader(var outer: Cursor) { fun read(cursor: Cursor) { outer = query(); while (cursor.moveToNext()) { outer.getColumnIndex("x") } } }`,
		`class Reader(val outer: Cursor) { fun read(cursors: List<Cursor>) { for (outer in cursors) { while (outer.moveToNext()) { outer.getColumnIndex("x") } } } }`,
	} {
		if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 0 {
			t.Fatalf("expected silence without an enclosing outer binding, got %v for %s", findings, code)
		}
	}
}

func TestCursorLoopJavaShadowedField(t *testing.T) {
	for _, body := range []string{
		`while (cursor.moveToNext()) { if (next instanceof Box(Cursor row)) { row.getColumnIndex("name"); } }`,
		`while (cursor.moveToNext()) { switch (next) { case Box(Cursor row) -> row.getColumnIndex("name"); } }`,
		`while (cursor.moveToNext()) { if (next instanceof Cursor row) { row.getColumnIndex("name"); } }`,
		`if (next instanceof Cursor row) { while (row.moveToNext()) { row.getColumnIndex("name"); } }`,
		`while (cursor.moveToNext()) { switch (next) { case Cursor row -> row.getColumnIndex("name"); } }`,
		`for (Cursor row : cursors) { while (row.moveToNext()) { row.getColumnIndex("name"); } }`,
		`try (Cursor row = query()) { while (row.moveToNext()) { row.getColumnIndex("name"); } }`,
		`row = query(); while (cursor.moveToNext()) { row.getColumnIndex("name"); }`,
		`while (cursor.moveToNext()) { this.row = query(); row.getColumnIndex("name"); }`,
	} {
		code := `class Reader { Cursor row; void read(Cursor cursor, Object next) { ` + body + ` } }`
		if findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 0 {
			t.Fatalf("expected silence for shadowed/written field, got %v for %s", findings, code)
		}
	}
}

func TestCursorLoopJavaPositiveFixture(t *testing.T) {
	path := filepath.Join(fixtureRoot(t), "java-android-support", "java-only-app", "src", "main", "java", "com", "example", "javaonly", "MainActivity.java")
	code, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", string(code))
	if len(findings) != 1 || findings[0].Line != 31 {
		t.Fatalf("expected Java positive fixture lookup at line 31, got %v", findings)
	}
}

func TestCursorLoopColumnArguments(t *testing.T) {
	cases := []struct {
		name string
		code string
		want int
	}{
		{"literal", `fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex("name") }`, 1},
		{"raw literal", `fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex("""name""") }`, 1},
		{"parameter", `fun read(c: Cursor, col: String) { while (c.moveToNext()) c.getColumnIndex(col) }`, 1},
		{"val", `fun read(c: Cursor) { val col = loadName(); while (c.moveToNext()) c.getColumnIndex(col) }`, 1},
		{"destructured val", `fun read(c: Cursor) { val (col, other) = names; while (c.moveToNext()) c.getColumnIndex(col) }`, 1},
		{"constant miss", `const val COLUMN = "name"; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(COLUMN) }`, 0},
		{"later constant miss", `fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(COLUMN) }; const val COLUMN = "name"`, 0},
		{"companion constant miss", `class Reader { companion object { const val COLUMN = "name" }; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(COLUMN) } }`, 0},
		{"object constant miss", `object Reader { const val COLUMN = "name"; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(COLUMN) } }`, 0},
		{"extension getter shadow", `const val COLUMN = "name"; class Columns { val COLUMN: String get() = pick() }; fun Columns.read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(COLUMN) }`, 0},
		{"receiver lambda getter shadow", `const val COLUMN = "name"; fun read(c: Cursor, columns: Columns) { with(columns) { val rows = c; while (rows.moveToNext()) rows.getColumnIndex(COLUMN) } }`, 0},
		{"later member getter shadow", `const val COLUMN = "name"; class Reader { fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(COLUMN) }; val COLUMN: String get() = pick() }`, 0},
		{"companion getter shadow", `const val COLUMN = "name"; class Reader { companion object { val COLUMN: String get() = pick() }; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(COLUMN) } }`, 0},
		{"primary constructor var shadow", `const val COLUMN = "name"; class Reader(var COLUMN: String) { fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(COLUMN) } }`, 0},
		{"primary constructor open val shadow", `const val COLUMN = "name"; open class Reader(open val COLUMN: String) { fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(COLUMN) } }`, 0},
		{"inherited getter shadow", `const val COLUMN = "name"; class Reader : Base() { fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(COLUMN) } }`, 0},
		{"mutable", `fun read(c: Cursor) { var col = "name"; while (c.moveToNext()) c.getColumnIndex(col) }`, 0},
		{"destructured var", `fun read(c: Cursor) { var (col, other) = names; while (c.moveToNext()) c.getColumnIndex(col) }`, 0},
		{"qualified constant miss", `object Columns { const val NAME = "name" }; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Columns.NAME) }`, 0},
		{"computed constant miss", `const val COLUMN = "na" + "me"; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(COLUMN) }`, 0},
		{"written mutable", `fun read(c: Cursor, first: String) { var col = first; while (c.moveToNext()) { c.getColumnIndex(col); col = c.getString(0) } }`, 0},
		{"row call", `fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(c.getString(0)) }`, 0},
		{"arbitrary call", `fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(loadName()) }`, 0},
		{"indexed", `fun read(c: Cursor, columns: List<String>) { var i = 0; while (c.moveToNext()) { c.getColumnIndex(columns[i]); i++ } }`, 0},
		{"template", `fun read(c: Cursor, col: String) { while (c.moveToNext()) c.getColumnIndex("$col") }`, 0},
		{"row template", `fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex("${c.getString(0)}") }`, 0},
		{"concatenation", `fun read(c: Cursor, col: String) { while (c.moveToNext()) c.getColumnIndex(col + "suffix") }`, 0},
		{"property", `val col = "name"; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(col) }`, 0},
		{"delegated val", `fun read(c: Cursor) { val col by lazy { "name" }; while (c.moveToNext()) c.getColumnIndex(col) }`, 0},
		{"captured argument", `fun read(c: Cursor, col: String) { fun inner(rows: Cursor) { while (rows.moveToNext()) rows.getColumnIndex(col) }; inner(c) }`, 0},
		{"constant shadow", `const val col = "name"; fun read(c: Cursor) { var col = "other"; while (c.moveToNext()) c.getColumnIndex(col) }`, 0},
		{"nested literal", `fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(c.getColumnIndex("inner").toString()) }`, 1},
		{"anonymous function", `val read = fun(rows: Cursor) { while (rows.moveToNext()) rows.getColumnIndex("name") }`, 1},
		{"same function name", `fun rows(rows: Cursor) { while (rows.moveToNext()) rows.getColumnIndex("name") }`, 1},
		{"later shadow", `fun read(c: Cursor, rows: Cursor, ok: Boolean) { if (ok) { while (c.moveToNext()) rows.getColumnIndex("name") }; val rows = c }`, 1},
		{"secondary constructor", `class Reader { constructor(c: Cursor) { while (c.moveToNext()) c.getColumnIndex("name") } }`, 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", test.code); len(findings) != test.want {
				t.Fatalf("expected %d, got %v", test.want, findings)
			}
		})
	}
}

func TestCursorLoopTryCatchBindings(t *testing.T) {
	cases := []struct {
		name string
		code string
	}{
		{"catch shadows receiver", `fun read(c: Cursor) {
    try { while (c.moveToNext()) c.getColumnIndex("name") } catch (c: Exception) {}
}`},
		{"catch shadows argument", `fun read(c: Cursor, e: String) {
    try { while (c.moveToNext()) c.getColumnIndex(e) } catch (e: Exception) {}
}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", test.code)
			if len(findings) != 1 || findings[0].Line != 2 {
				t.Fatalf("expected one finding on line 2, got %v", findings)
			}
		})
	}
}

func TestCursorLoopJavaInheritedColumnShadow(t *testing.T) {
	code := `class Base { String COLUMN; }
class Reader { static final String COLUMN = "name";
static class Inner extends Base {
void read(Cursor c) { while (c.moveToNext()) { c.getColumnIndex(COLUMN); } }
} }`
	if findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 0 {
		t.Fatalf("inherited mutable field must not bind to outer constant: %v", findings)
	}
}

func TestCursorLoopJavaColumnArguments(t *testing.T) {
	cases := []struct {
		body string
		want int
	}{
		{`while (c.moveToNext()) { c.getColumnIndex(col); }`, 1},
		{`String name = loadName(); while (c.moveToNext()) { c.getColumnIndex(name); }`, 1},
		{`while (c.moveToNext()) { c.getColumnIndex(COLUMN); }`, 0},
		{`while (c.moveToNext()) { c.getColumnIndex(Reader.COLUMN); }`, 0},
		{`while (c.moveToNext()) { c.getColumnIndex(RUNTIME_COLUMN); }`, 0},
		{`while (c.moveToNext()) { c.getColumnIndex(OTHER_COLUMN); }`, 0},
		{`while (c.moveToNext()) { c.getColumnIndex(col); col = c.getString(0); }`, 0},
		{`col = "other"; while (c.moveToNext()) { c.getColumnIndex(col); }`, 0},
		{`while (c.moveToNext()) { c.getColumnIndex(col); } col = "other";`, 0},
		{`String name = "first"; while (c.moveToNext()) { c.getColumnIndex(name); name = c.getString(0); }`, 0},
		{`while (c.moveToNext()) { c.getColumnIndex(c.getString(0)); }`, 0},
		{`while (c.moveToNext()) { c.getColumnIndex(loadName()); }`, 0},
		{`int i = 0; while (c.moveToNext()) { c.getColumnIndex(columns[i]); i++; }`, 0},
		{`while (c.moveToNext()) { c.getColumnIndex(col + "suffix"); }`, 0},
		{`String COLUMN = loadName(); while (c.moveToNext()) { c.getColumnIndex(COLUMN); COLUMN = c.getString(0); }`, 0},
	}
	for _, test := range cases {
		t.Run(test.body, func(t *testing.T) {
			code := `class Reader { static final String COLUMN = "name", OTHER_COLUMN = loadName(); static final String RUNTIME_COLUMN = loadName(); void read(Cursor c, String col) { ` + test.body + ` } }`
			if findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != test.want {
				t.Fatalf("expected %d, got %v", test.want, findings)
			}
		})
	}
}
