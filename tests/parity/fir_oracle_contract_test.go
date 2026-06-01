package parity_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
)

// TestFirOracleFactContract is the FIR-side safety gate for making the
// FIR backend the default oracle.
//
// The pre-existing TestOracleBackendParity gate only compares the
// *declaration* surface (class kind, supertypes, sealed/data/abstract
// flags, member names) across KAA and FIR. It deliberately says nothing
// about the *expression* surface — expression types, nullability,
// resolved call targets, suspend markers, and compiler diagnostics —
// because the two backends diverge structurally there in one-shot mode
// (KAA emits expressions only for filter-requested positions; FIR
// populates them eagerly). Those expression facts are exactly what the
// type-aware rules read.
//
// Rather than pin FIR's behaviour to the backend being retired, this
// test asserts FIR delivers each load-bearing fact directly, against a
// fixture with known shapes. If a krit-fir change silently drops
// expression types, nullability projection, call-target resolution,
// suspend detection, or diagnostics, this fails before the default
// flip can ship a regression.
//
// Assertions key on semantic criteria (call-target FQN suffix, declared
// type, nullability) rather than brittle line:col offsets, so the gate
// survives fixture reformatting and renderer drift.
func TestFirOracleFactContract(t *testing.T) {
	root := repoRoot(t)
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar` to enable the FIR oracle contract")
	}
	stdlib := findKotlinStdlib()
	if stdlib == "" {
		t.Skip("kotlin-stdlib jar not found in Gradle cache; required for stdlib-typed contract fixture")
	}

	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "Contract.kt"), []byte(firContractFixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	opts := oracle.InvocationOptions{Classpath: []string{stdlib}}
	data := runOneShot(t, firJar, srcDir, filepath.Join(tmp, "fir.json"), opts)

	file := singleFile(t, data)
	exprs := file.Expressions

	// Fact 1: expression types are populated with stdlib-resolved FQNs.
	// Without expression types every NeedsOracleExprType rule is blind.
	assertHasExpressionType(t, exprs, "kotlin.String")
	assertHasExpressionType(t, exprs, "kotlin.Int")
	assertHasExpressionType(t, exprs, "kotlin.Boolean")

	// Fact 2: nullability projection. `maybe: String?` must surface as a
	// nullable expression; null-safety rules key off this bit.
	if !hasNullableExpression(exprs) {
		t.Errorf("no nullable expression found; FIR dropped nullability projection\nexpressions: %s", dumpExpressions(exprs))
	}

	// Fact 3: call-target resolution. `repo.fetch()` / `repo.plain()`
	// must resolve to the declared member FQNs, with callTargetResolved
	// set. NeedsOracleCallTargets rules depend on this.
	fetch := requireResolvedCallTarget(t, exprs, "fetch")
	plain := requireResolvedCallTarget(t, exprs, "plain")

	// Fact 4: suspend markers. The suspend member must carry
	// callTargetSuspend; the plain member must not. Coroutine rules
	// (e.g. blocking-call-in-suspend) read exactly this.
	if !fetch.CallTargetSuspend {
		t.Errorf("resolved call target for suspend `fetch` missing callTargetSuspend=true: %+v", fetch)
	}
	if plain.CallTargetSuspend {
		t.Errorf("resolved call target for non-suspend `plain` has callTargetSuspend=true: %+v", plain)
	}

	// Fact 5: compiler diagnostics. The USELESS_ELVIS on a non-null
	// receiver must be projected; diagnostic-backed rules read these.
	if !hasDiagnostic(file, "USELESS_ELVIS") {
		t.Errorf("expected USELESS_ELVIS diagnostic, got %s", dumpDiagnostics(file))
	}
}

// firContractFixture exercises each load-bearing FIR fact on one axis so
// a regression points at a category, not a blob:
//   - stdlib-typed locals (String/Int/Boolean) → expression types
//   - `String?` local → nullability projection
//   - suspend + non-suspend member calls → call-target + suspend marker
//   - elvis on a non-null receiver → USELESS_ELVIS diagnostic
const firContractFixture = `package contract

class Repo {
    suspend fun fetch(): String = "data"
    fun plain(): Int = 1
}

suspend fun caller(repo: Repo, flag: Boolean): Int {
    val s: String = "hello"
    val n: Int = s.length
    val data: String = repo.fetch()
    val p: Int = repo.plain()
    val maybe: String? = if (flag) data else null
    val len = maybe?.length ?: 0
    val nonNull: String = "always"
    val useless: String = nonNull ?: "fallback"
    return n + len + p + useless.length
}
`

func assertHasExpressionType(t *testing.T, exprs map[string]*oracle.ExpressionType, fqn string) {
	t.Helper()
	for _, e := range exprs {
		if e.Type == fqn {
			return
		}
	}
	t.Errorf("no expression with type %q; FIR dropped expression types\nexpressions: %s", fqn, dumpExpressions(exprs))
}

func hasNullableExpression(exprs map[string]*oracle.ExpressionType) bool {
	for _, e := range exprs {
		if e.Nullable {
			return true
		}
	}
	return false
}

// requireResolvedCallTarget finds the expression whose resolved call
// target ends in the given member name and asserts callTargetResolved.
// Matching on the tail segment keeps the assertion independent of the
// package/FQN renderer.
func requireResolvedCallTarget(t *testing.T, exprs map[string]*oracle.ExpressionType, member string) *oracle.ExpressionType {
	t.Helper()
	for _, e := range exprs {
		if e.CallTarget == "" {
			continue
		}
		if lastSegment(e.CallTarget) != member {
			continue
		}
		if !e.CallTargetResolved {
			t.Errorf("call target %q for %q not marked resolved: %+v", e.CallTarget, member, e)
		}
		return e
	}
	t.Fatalf("no resolved call target ending in %q; FIR dropped call-target resolution\nexpressions: %s", member, dumpExpressions(exprs))
	return nil
}

func hasDiagnostic(f *oracle.File, factory string) bool {
	for _, d := range f.Diagnostics {
		if d.FactoryName == factory {
			return true
		}
	}
	return false
}

func singleFile(t *testing.T, d *oracle.Data) *oracle.File {
	t.Helper()
	if len(d.Files) != 1 {
		t.Fatalf("expected exactly one file in oracle output, got %d", len(d.Files))
	}
	for _, f := range d.Files {
		return f
	}
	return nil
}

func dumpExpressions(exprs map[string]*oracle.ExpressionType) string {
	parts := make([]string, 0, len(exprs))
	for _, e := range exprs {
		seg := e.Type
		if e.CallTarget != "" {
			seg += "->" + e.CallTarget
			if e.CallTargetSuspend {
				seg += "(suspend)"
			}
		}
		if e.Nullable {
			seg += "?"
		}
		parts = append(parts, seg)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func dumpDiagnostics(f *oracle.File) string {
	parts := make([]string, 0, len(f.Diagnostics))
	for _, d := range f.Diagnostics {
		parts = append(parts, d.FactoryName)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
