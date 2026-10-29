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

// TestOracleDeprecationDiagnostics requires the FIR oracle to report a
// DEPRECATION diagnostic for every reference kind, in a use-site file that
// contains none of the null-safety or control-flow tokens a lexical pre-gate
// could key on, and none for the non-deprecated lookalikes.
func TestOracleDeprecationDiagnostics(t *testing.T) {
	root := repoRoot(t)
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `make fir-jar`")
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
	data := runOneShot(t, firJar, srcDir, filepath.Join(tmp, "krit-fir.json"), opts)
	use := indexByBasename(data.Files)["Use.kt"]
	if use == nil {
		t.Fatal("no facts for Use.kt")
	}
	var got []int
	for _, d := range use.Diagnostics {
		if d.FactoryName == "DEPRECATION" {
			got = append(got, d.Line)
		}
	}
	sort.Ints(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DEPRECATION lines = %v, want %v", got, want)
	}
}
