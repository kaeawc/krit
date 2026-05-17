package evidence

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// TestResolveOwnerKotlinPrimaryConstructor exercises the AST fallback that
// covers `class Foo(private val db: SQLiteDatabase)` — a shape the
// in-process source resolver does not (yet) populate as a scope entry.
func TestResolveOwnerKotlinPrimaryConstructor(t *testing.T) {
	file := parseKotlin(t, `
package test
import android.database.sqlite.SQLiteDatabase
class UserDao(private val db: SQLiteDatabase) {
    fun load() {
        db.rawQuery("SELECT 1", null)
    }
}
`)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	ev := &Evidence{file: file, resolver: resolver}

	call := firstCall(t, file, "rawQuery")
	c := ev.Call(call)
	if c == nil || c.Callee != "rawQuery" || c.Receiver != "db" {
		t.Fatalf("unexpected call shape: %+v", c)
	}
	fqn, src := ev.ResolveOwner(c)
	if src == OwnerUnknown {
		t.Fatalf("expected a resolved owner, got OwnerUnknown")
	}
	if fqn != "android.database.sqlite.SQLiteDatabase" {
		t.Fatalf("got fqn=%q, want android.database.sqlite.SQLiteDatabase", fqn)
	}
}

// TestResolveOwnerKotlinFunctionParameter covers function-parameter
// receivers such as `fun load(db: SQLiteDatabase)`.
func TestResolveOwnerKotlinFunctionParameter(t *testing.T) {
	file := parseKotlin(t, `
package test
import android.database.sqlite.SQLiteDatabase
class UserDao {
    fun load(db: SQLiteDatabase) {
        db.execSQL("DELETE FROM x")
    }
}
`)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	ev := &Evidence{file: file, resolver: resolver}

	call := firstCall(t, file, "execSQL")
	c := ev.Call(call)
	fqn, src := ev.ResolveOwner(c)
	if src == OwnerUnknown || fqn != "android.database.sqlite.SQLiteDatabase" {
		t.Fatalf("got (%q, %d), want (android.database.sqlite.SQLiteDatabase, nonzero)", fqn, src)
	}
}

// TestResolveOwnerLocalLookalikeRejected guarantees a local class with the
// same method name does NOT resolve to the unrelated FQN — the previous
// substring-match implementation was vulnerable to this.
func TestResolveOwnerLocalLookalikeRejected(t *testing.T) {
	file := parseKotlin(t, `
package test
class QueryRunner { fun rawQuery(sql: String, args: Array<String>?) {} }
class UserDao(private val db: QueryRunner) {
    fun load() { db.rawQuery("x", null) }
}
`)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	ev := &Evidence{file: file, resolver: resolver}

	call := firstCall(t, file, "rawQuery")
	c := ev.Call(call)
	fqn, src := ev.ResolveOwner(c)
	// We don't require a particular fqn for a local type — but it must NOT
	// be the SQLiteDatabase FQN.
	if fqn == "android.database.sqlite.SQLiteDatabase" {
		t.Fatalf("local QueryRunner falsely resolved to SQLiteDatabase (src=%d)", src)
	}
}

func parseKotlin(t *testing.T, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func firstCall(t *testing.T, file *scanner.File, callee string) uint32 {
	t.Helper()
	var found uint32
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if found != 0 {
			return
		}
		ev := &Evidence{file: file}
		c := ev.Call(idx)
		if c != nil && c.Callee == callee {
			found = idx
		}
	})
	if found == 0 {
		t.Fatalf("call %q not found", callee)
	}
	return found
}
