package parity_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
)

// TestOracleBackendDeprecationParity requires both oracle backends to report the
// same DEPRECATION diagnostics for every reference kind, in a use-site file that
// contains none of the null-safety or control-flow tokens krit-types once used
// as a lexical pre-gate. With that gate, krit-types collected no diagnostics
// for such a file while krit-fir reported every deprecation.
func TestOracleBackendDeprecationParity(t *testing.T) {
	root := repoRoot(t)
	kaaJar := oracle.FindJar([]string{root})
	if kaaJar == "" {
		t.Skip("krit-types jar not found; run `cd tools/krit-types && ./gradlew shadowJar`")
	}
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	stdlib := findKotlinStdlib()
	if stdlib == "" {
		t.Skip("kotlin-stdlib jar not found in Gradle cache")
	}

	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{
		"Lib.kt": `package dep

@Deprecated("use newFn") fun oldFn() {}
fun overloaded(i: Int) {}
@Deprecated("use the Int overload") fun overloaded(s: String) {}
@Deprecated("use NewType") class OldType
class Holder {
    @Deprecated("use newProp") val oldProp: Int = 1
    val newProp: Int = 2
}
open class Base { @Deprecated("gone") open fun m() {} }
class Child : Base()
@Deprecated("use String") typealias OldAlias = String
`,
		"Use.kt": `package dep

import java.util.Date

fun useAll(h: Holder, c: Child, t: OldType, a: OldAlias) {
    oldFn()
    overloaded(1)
    overloaded("s")
    println(h.oldProp)
    println(h.newProp)
    c.m()
    println(Date().year)
}
`,
	}
	for name, body := range sources {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Line 5 carries two (the OldType and OldAlias parameter types); then
	// oldFn, the String overload, oldProp, the inherited m, and the
	// Java-deprecated Date.year getter. Never overloaded(1) or newProp.
	want := []int{5, 5, 6, 8, 9, 11, 12}
	opts := oracle.InvocationOptions{Classpath: []string{stdlib}}
	for _, backend := range []struct{ name, jar string }{{"krit-types", kaaJar}, {"krit-fir", firJar}} {
		data := runOneShot(t, backend.jar, srcDir, filepath.Join(tmp, backend.name+".json"), opts)
		use := indexByBasename(data.Files)["Use.kt"]
		if use == nil {
			t.Fatalf("%s: no facts for Use.kt", backend.name)
		}
		var got []int
		for _, d := range use.Diagnostics {
			if d.FactoryName == "DEPRECATION" {
				got = append(got, d.Line)
			}
		}
		sort.Ints(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: DEPRECATION lines = %v, want %v", backend.name, got, want)
		}
	}
}
