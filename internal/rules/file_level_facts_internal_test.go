package rules

import (
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/filefacts"
)

// #944: these per-file facts are asked for from every node the rules visit,
// so they are computed once per file and reused.
func TestFileLevelFactsAreComputedOncePerFile(t *testing.T) {
	prev := fileFactsCache()
	setSharedFileFacts(filefacts.NewCache())
	t.Cleanup(func() { setSharedFileFacts(prev) })

	file := parseCoroutinesInline(t, `package p

import javax.crypto.Cipher
import androidx.compose.material.Icon

class Foo {
    class Inner
}
object Bar
typealias Baz = String

fun use(c: Cipher) = Foo()
`)
	names := fileDeclaredTypeNames(file)
	for _, want := range []string{"Foo", "Inner", "Bar", "Baz"} {
		if !names[want] || !staticIvFileDeclaresType(file, want) {
			t.Errorf("%s should be a declared type: %v", want, names)
		}
	}
	if staticIvFileDeclaresType(file, "Cipher") || staticIvFileDeclaresType(file, "use") {
		t.Errorf("Cipher and use are not declared types here: %v", names)
	}
	if reflect.ValueOf(fileDeclaredTypeNames(file)).Pointer() != reflect.ValueOf(names).Pointer() {
		t.Error("declared type names were collected again for the same file")
	}

	if !prngFromSystemTimeCryptoFile(file) {
		t.Error("the file imports javax.crypto")
	}
	first := composeContentDescriptionImports(file)
	if !first.hasRelevant || !composeContentDescriptionFileMayUse(file) {
		t.Error("the file imports a Compose material Icon")
	}
	if reflect.ValueOf(composeContentDescriptionImports(file).wildcards).Pointer() != reflect.ValueOf(first.wildcards).Pointer() {
		t.Error("compose import facts were computed again for the same file")
	}

	plain := parseCoroutinesInline(t, "package p\n\nfun f() = 1\n")
	if prngFromSystemTimeCryptoFile(plain) || composeContentDescriptionFileMayUse(plain) || len(fileDeclaredTypeNames(plain)) != 0 {
		t.Error("a file with no crypto imports, Compose imports or types has none of the facts")
	}
}
