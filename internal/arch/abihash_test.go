package arch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func parseSource(t *testing.T, src string) *scanner.File {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "F.kt")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return f
}

func hashOf(t *testing.T, src string) string {
	t.Helper()
	f := parseSource(t, src)
	return HashAbiSignatures(ExtractAbiSignatures([]*scanner.File{f}))
}

func TestAbiHash_StableAcrossBodyEdits(t *testing.T) {
	a := `package p

class Foo {
    fun greet(name: String): String {
        return "hi " + name
    }
}`
	b := `package p

// changed comment
class Foo {
    fun greet(name: String): String {
        // different body, same signature
        val msg = "hello"
        return msg + " " + name
    }
}`
	if hashOf(t, a) != hashOf(t, b) {
		t.Fatalf("body/comment edit changed the ABI hash")
	}
}

func TestAbiHash_StableAcrossPrivateHelperEdits(t *testing.T) {
	a := `package p

class Foo {
    fun publicApi(): Int = 1
    private fun helper(): Int = 2
}`
	b := `package p

class Foo {
    fun publicApi(): Int = 1
    private fun helper(): Int = 99
    private fun anotherHelper(): String = ""
}`
	if hashOf(t, a) != hashOf(t, b) {
		t.Fatalf("private helper edits changed the ABI hash")
	}
}

func TestAbiHash_StableAcrossParamNameRename(t *testing.T) {
	a := `package p

fun greet(name: String): String = name`
	b := `package p

fun greet(other: String): String = other`
	if hashOf(t, a) != hashOf(t, b) {
		t.Fatalf("parameter rename changed the ABI hash")
	}
}

func TestAbiHash_ChangesOnReturnType(t *testing.T) {
	a := `package p

fun f(): Int = 0`
	b := `package p

fun f(): Long = 0L`
	if hashOf(t, a) == hashOf(t, b) {
		t.Fatalf("return type change did not change the ABI hash")
	}
}

func TestAbiHash_ChangesOnNewPublicFunction(t *testing.T) {
	a := `package p

class Foo {
    fun a(): Int = 0
}`
	b := `package p

class Foo {
    fun a(): Int = 0
    fun b(): Int = 0
}`
	if hashOf(t, a) == hashOf(t, b) {
		t.Fatalf("adding a public function did not change the ABI hash")
	}
}

func TestAbiHash_ChangesOnDefaultPresence(t *testing.T) {
	a := `package p

fun f(x: Int): Int = x`
	b := `package p

fun f(x: Int = 0): Int = x`
	if hashOf(t, a) == hashOf(t, b) {
		t.Fatalf("adding a default did not change the ABI hash")
	}
}

func TestAbiHash_StableAcrossDefaultValueChange(t *testing.T) {
	a := `package p

fun f(x: Int = 0): Int = x`
	b := `package p

fun f(x: Int = 42): Int = x`
	if hashOf(t, a) != hashOf(t, b) {
		t.Fatalf("default value change changed the ABI hash (only presence should matter)")
	}
}

func TestAbiHash_HashFormat(t *testing.T) {
	h := hashOf(t, `package p

fun f(): Int = 0`)
	if len(h) != 16 {
		t.Fatalf("expected 16-char hex, got %q (%d)", h, len(h))
	}
}

func TestAbiHash_EmptyDeterministic(t *testing.T) {
	h1 := HashAbiSignatures(nil)
	h2 := HashAbiSignatures(nil)
	if h1 != h2 {
		t.Fatalf("empty hash not deterministic: %s vs %s", h1, h2)
	}
}

func TestAbiHash_AllowedAnnotationChangesHash(t *testing.T) {
	a := `package p

class Foo {
    fun f(): Int = 0
}`
	b := `package p

class Foo {
    @JvmStatic fun f(): Int = 0
}`
	if hashOf(t, a) == hashOf(t, b) {
		t.Fatalf("@JvmStatic addition did not change the hash")
	}
}

func TestAbiHash_NonAllowlistedAnnotationStable(t *testing.T) {
	a := `package p

class Foo {
    fun f(): Int = 0
}`
	b := `package p

class Foo {
    @VisibleForTesting fun f(): Int = 0
}`
	if hashOf(t, a) != hashOf(t, b) {
		t.Fatalf("@VisibleForTesting changed the hash; should be ignored")
	}
}
