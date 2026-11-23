package parity_test

import "testing"

func TestCursorLoopLabeledLambdaAndLoop(t *testing.T) {
	sources := map[string]string{
		"Query.kt": `package cursorloop.labels
import android.database.Cursor

fun labeledTrailingLambda(cursors: List<Cursor>) {
    cursors.forEach outer@{ cursor -> while (cursor.moveToNext()) cursor.getColumnIndex("name") }
}

fun labeledWhileLoop(cursor: Cursor) {
    cursor.use { row -> rows@ while (row.moveToNext()) row.getColumnIndex("name") }
}

fun annotatedTrailingLambda(cursors: List<Cursor>) {
    cursors.forEach @Suppress("UNUSED_VARIABLE") { cursor -> while (cursor.moveToNext()) cursor.getColumnIndex("name") }
}

fun annotatedWhileLoop(cursor: Cursor) {
    cursor.use { row -> @Suppress("UNUSED_VARIABLE") while (row.moveToNext()) row.getColumnIndex("name") }
}
`,
	}
	findings := firCheck(t, []string{"CursorLoopWithColumnIndexInLoop"}, sources)["Query.kt"]
	want := []int{5, 9, 13, 17}
	if len(findings) != len(want) {
		t.Fatalf("expected all labeled and annotated loop lookups, got %v", findings)
	}
	for index, line := range want {
		if findings[index].Line != line {
			t.Errorf("finding %d: expected line %d, got %v", index, line, findings[index])
		}
	}
}

func TestCursorLoopLibraryLambdaConstants(t *testing.T) {
	sources := map[string]string{
		"Query.kt": `package cursorloop.library
import android.database.Cursor
import android.database.sqlite.SQLiteDatabase
import android.provider.Settings
import android.provider.Settings.Global.AIRPLANE_MODE_ON

fun read(db: SQLiteDatabase, cursors: List<Cursor>) {
    db.query("items", null, null, null, null, null, null)?.use { cursor ->
        while (cursor.moveToNext()) {
            val value = cursor.getString(cursor.getColumnIndexOrThrow(Settings.Global.AIRPLANE_MODE_ON))
        }
    }
    db.rawQuery("SELECT name FROM items", null)?.let { cursor ->
        while (cursor.moveToNext()) cursor.getColumnIndex(Settings.Global.AIRPLANE_MODE_ON)
    }
    db.rawQuery("SELECT name FROM items", null)?.also { cursor ->
        while (cursor.moveToNext()) cursor.getColumnIndex(AIRPLANE_MODE_ON)
    }
    cursors.forEach { cursor ->
        while (cursor.moveToNext()) cursor.getColumnIndex(Settings.Global.AIRPLANE_MODE_ON)
    }
}
`,
	}
	findings := firCheck(t, []string{"CursorLoopWithColumnIndexInLoop"}, sources)["Query.kt"]
	want := []int{10, 14, 17, 20}
	if len(findings) != len(want) {
		t.Fatalf("expected four library-backed column lookups, got %v", findings)
	}
	for index, line := range want {
		if findings[index].Line != line {
			t.Errorf("finding %d: expected line %d, got %v", index, line, findings[index])
		}
	}
}
