package parity_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

// kotlin2xSmartCastSource holds K2 smart casts that K1 (and so detekt) cannot see. A marker comment names what
// the compiler must say about the line: `// safe` is an UNNECESSARY_SAFE_CALL, `// bang` an
// UNNECESSARY_NOT_NULL_ASSERTION, `// recv` a member access whose receiver carries the smart-cast type, and
// `// keep` a line that needs its `?.` or `!!` and must stay silent.
const kotlin2xSmartCastSource = `package smartcast

class Box(val stable: String?, var mutable: String?)

fun booleanFlag(x: String?) {
    val isNull = x == null
    if (!isNull) {
        x?.length // safe
        x!!.length // bang
    }
}

fun orEarlyExit(x: String?) {
    if (x == null || x.isEmpty()) return
    x?.length // safe
}

fun commonSupertype(x: Any) {
    if (x is String || x is CharSequence) {
        x.length // keep
    }
}

fun localVarInInlineLambda(x: String?) {
    var y = x
    if (y != null) {
        run { y?.length } // safe
    }
}

fun stableProperty(b: Box) {
    if (b.stable != null) {
        b.stable?.length // safe
        b.stable!!.length // bang
    }
    if (b.mutable != null) {
        b.mutable?.length // keep
    }
}

fun finallyBlock(x: String?) {
    try {
        println()
    } finally {
        x?.length // keep
    }
}

fun contract(x: String?) {
    require(x != null)
    x?.length // safe
    x!!.length // bang
}

fun conjunction(x: Int?) {
    if (x != null && x > 1) {
        x!!.toString() // bang
    }
}

fun smartCastReceiver(x: Any?) {
    if (x is String) {
        x.length // recv
    }
}
`

// TestOracleKotlin2xSmartCasts holds the oracle to the K2 compiler on smart casts the structural view cannot
// follow: its null-safety diagnostics land on exactly the marked lines, and the receiver of a call keeps its
// smart-cast type even though the call shares its start position.
func TestOracleKotlin2xSmartCasts(t *testing.T) {
	jar, stdlib := kotlin24FirJar(t)
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(srcDir, "SmartCasts.kt"), kotlin2xSmartCastSource)
	data := runOneShot(t, jar, srcDir, filepath.Join(tmp, "oracle.json"), oracle.InvocationOptions{Classpath: []string{stdlib}})

	var file *oracle.File
	for path, f := range data.Files {
		if filepath.Base(path) == "SmartCasts.kt" {
			file = f
		}
	}
	if file == nil {
		t.Fatal("SmartCasts.kt was not analyzed")
	}

	want := map[string][]int{}
	for i, line := range strings.Split(kotlin2xSmartCastSource, "\n") {
		switch {
		case strings.HasSuffix(line, "// safe"):
			want["UNNECESSARY_SAFE_CALL"] = append(want["UNNECESSARY_SAFE_CALL"], i+1)
		case strings.HasSuffix(line, "// bang"):
			want["UNNECESSARY_NOT_NULL_ASSERTION"] = append(want["UNNECESSARY_NOT_NULL_ASSERTION"], i+1)
		}
	}
	got := map[string][]int{}
	for _, d := range file.Diagnostics {
		switch d.FactoryName {
		case "UNNECESSARY_SAFE_CALL", "UNNECESSARY_NOT_NULL_ASSERTION":
			got[d.FactoryName] = append(got[d.FactoryName], d.Line)
		}
	}
	for _, lines := range got {
		sort.Ints(lines)
	}
	for factory, lines := range want {
		if !equalInts(lines, got[factory]) {
			t.Errorf("%s: want lines %v, got %v", factory, lines, got[factory])
		}
	}
	for factory := range got {
		if _, ok := want[factory]; !ok {
			t.Errorf("unexpected %s on lines %v", factory, got[factory])
		}
	}

	for i, line := range strings.Split(kotlin2xSmartCastSource, "\n") {
		if !strings.HasSuffix(line, "// recv") {
			continue
		}
		found := false
		for pos, e := range file.Expressions {
			if positionLine(pos) == i+1 && strings.Contains(pos, "@") && e.Type == "kotlin.String" && !e.Nullable {
				found = true
			}
		}
		if !found {
			t.Errorf("line %d: no range-keyed smart-cast fact (non-null kotlin.String) for the call receiver", i+1)
		}
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
