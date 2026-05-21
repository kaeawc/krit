package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
)

// TestOracleBackendParity is the CI gate guarding KAA ↔ FIR
// divergence on the type-oracle wire shape. Both backends are
// invoked through the one-shot CLI on a multi-file fixture covering
// the Kotlin shapes most likely to expose renderer drift (generics,
// sealed/enum/data, objects, type aliases, inline-reified,
// intersection bounds). Tolerances live in the helpers below.
func TestOracleBackendParity(t *testing.T) {
	root := repoRoot(t)
	kaaJar := oracle.FindJar([]string{root})
	if kaaJar == "" {
		t.Skip("krit-types jar not found; run `cd tools/krit-types && ./gradlew shadowJar` to enable oracle parity")
	}
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar` to enable oracle parity")
	}
	stdlib := findKotlinStdlib()
	if stdlib == "" {
		t.Skip("kotlin-stdlib jar not found in Gradle cache; required for stdlib-typed parity fixture")
	}

	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, body := range parityFixtureSources() {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	opts := oracle.InvocationOptions{Classpath: []string{stdlib}}
	kaaData := runOneShot(t, kaaJar, srcDir, filepath.Join(tmp, "kaa.json"), opts)
	firData := runOneShot(t, firJar, srcDir, filepath.Join(tmp, "fir.json"), opts)

	assertOracleParity(t, kaaData, firData)
}

// parityFixtureSources returns the source files that exercise the
// declaration shapes most likely to expose backend-rendering drift.
// Each file targets one axis so a regression points to a category,
// not a 200-line blob.
func parityFixtureSources() map[string]string {
	return map[string]string{
		"Basics.kt": `package parity

open class Greeter {
    fun greet(name: String): String = "hi, " + name
}

class LoudGreeter : Greeter()
`,
		"Generics.kt": `package parity

open class Box<T>(val value: T) {
    fun unwrap(): T = value
}

class StringBox(s: String) : Box<String>(s)
`,
		"Sealed.kt": `package parity

sealed class Shape {
    abstract fun area(): Double
}

data class Circle(val radius: Double) : Shape() {
    override fun area(): Double = 3.14 * radius * radius
}

data class Rectangle(val w: Double, val h: Double) : Shape() {
    override fun area(): Double = w * h
}
`,
		"Enums.kt": `package parity

enum class Direction {
    NORTH, SOUTH, EAST, WEST;

    fun describe(): String = name.lowercase()
}
`,
		// Counter + companion intentionally omitted: KAA hides
		// nested companions from `declarations` while FIR surfaces
		// them as separate FQNs. That divergence is tracked outside
		// this parity gate.
		"Objects.kt": `package parity

object Registry {
    fun register(id: String): Boolean = id.isNotEmpty()
}
`,
		// Inline-reified + type alias + intersection-bound use sites
		// are wrapped in classes because FIR projects a file
		// containing only top-level declarations with an empty
		// package, which is a separate (real) divergence and not
		// what this fixture is meant to exercise.
		"Inline.kt": `package parity

class TypeChecks {
    inline fun <reified T> isA(x: Any): Boolean = x is T
}
`,
		"Aliases.kt": `package parity

typealias Names = List<String>

class AliasUse {
    fun firstName(ns: Names): String = ns.first()
}
`,
		// Where-clause bounds aren't yet on the wire; the assertion
		// is just that the `T` type parameter survives projection.
		"Intersection.kt": `package parity

class Identity {
    fun <T> identity(x: T): T where T : Comparable<T>, T : CharSequence = x
}
`,
	}
}

func runOneShot(t *testing.T, jar, srcDir, outputPath string, opts oracle.InvocationOptions) *oracle.Data {
	t.Helper()
	if _, err := oracle.InvokeWithFilesWithOptions(jar, []string{srcDir}, outputPath, "", false, opts); err != nil {
		t.Fatalf("oracle invoke (%s): %v", jar, err)
	}
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read %s: %v", outputPath, err)
	}
	var data oracle.Data
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parse %s: %v\n%s", outputPath, err, string(raw))
	}
	return &data
}

// TestOracleBackendAcceptsClasspathFlag asserts that both backends'
// one-shot CLIs accept `--classpath` and analyze succeeds with the
// supplied JAR(s) on the classpath. Without this gate a typo on
// either side would break `oracle.InvokeWithFilesWithOptions` for
// every project that ships an explicit classpath.
//
// The classpath supplied is kotlin-stdlib (the dependency every
// Kotlin source set needs anyway). KAA's Analysis API ignores the
// flag in favor of its own classpath wiring; FIR uses it directly to
// configure K2's compilation environment. Both must finish without
// error.
func TestOracleBackendAcceptsClasspathFlag(t *testing.T) {
	root := repoRoot(t)
	kaaJar := oracle.FindJar([]string{root})
	if kaaJar == "" {
		t.Skip("krit-types jar not found")
	}
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir jar not found")
	}
	stdlib := findKotlinStdlib()
	if stdlib == "" {
		t.Skip("kotlin-stdlib jar not found in Gradle cache")
	}

	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "Sample.kt"), []byte("package x\nclass Sample\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	for _, tc := range []struct {
		name string
		jar  string
	}{
		{name: "kaa", jar: kaaJar},
		{name: "fir", jar: firJar},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(tmp, tc.name+".json")
			opts := oracle.InvocationOptions{Classpath: []string{stdlib}}
			if _, err := oracle.InvokeWithFilesWithOptions(tc.jar, []string{srcDir}, out, "", false, opts); err != nil {
				t.Fatalf("%s with --classpath: %v", tc.name, err)
			}
			raw, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("read %s output: %v", tc.name, err)
			}
			var data oracle.Data
			if err := json.Unmarshal(raw, &data); err != nil {
				t.Fatalf("parse %s: %v\n%s", tc.name, err, string(raw))
			}
			if data.Version != 1 {
				t.Errorf("%s version = %d, want 1", tc.name, data.Version)
			}
		})
	}
}

func assertOracleParity(t *testing.T, kaa, fir *oracle.Data) {
	t.Helper()
	// Version + kotlinVersion are baked into both backends at
	// compile time; if they diverge the wire shape has shifted.
	if kaa.Version != fir.Version {
		t.Errorf("version mismatch: kaa=%d fir=%d", kaa.Version, fir.Version)
	}
	if kaa.KotlinVersion != fir.KotlinVersion {
		t.Errorf("kotlinVersion mismatch: kaa=%q fir=%q", kaa.KotlinVersion, fir.KotlinVersion)
	}

	// Each backend keys `files` by absolute path. The paths may
	// differ in symlink normalization (KAA uses PSI virtual file
	// paths; FIR canonicalizes via java.io.File). Compare by file
	// basename so the test stays robust against `/var` ↔ `/private/var`
	// drift on macOS.
	kaaByBase := indexByBasename(kaa.Files)
	firByBase := indexByBasename(fir.Files)
	if !sameKeys(kaaByBase, firByBase) {
		t.Fatalf("files keys differ: kaa=%v fir=%v", sortedKeys(kaaByBase), sortedKeys(firByBase))
	}

	for base := range kaaByBase {
		assertFileParity(t, base, kaaByBase[base], firByBase[base])
	}
}

func assertFileParity(t *testing.T, name string, kaa, fir *oracle.File) {
	t.Helper()
	if kaa.Package != fir.Package {
		t.Errorf("%s: package mismatch: kaa=%q fir=%q", name, kaa.Package, fir.Package)
	}

	kaaDecls := indexClassesByFqn(kaa.Declarations)
	firDecls := indexClassesByFqn(fir.Declarations)
	if !sameKeys(kaaDecls, firDecls) {
		t.Fatalf("%s: declaration FQN set differs: kaa=%v fir=%v", name, sortedKeys(kaaDecls), sortedKeys(firDecls))
	}

	for fqn, kaaCls := range kaaDecls {
		firCls := firDecls[fqn]
		assertClassParity(t, name, fqn, kaaCls, firCls)
	}
}

func assertClassParity(t *testing.T, file, fqn string, kaa, fir *oracle.Class) {
	t.Helper()
	// KAA emits `kind="sealed class"`, FIR emits `kind="class"` +
	// `isSealed=true`. The isSealed check below catches drift in
	// the boolean; here we just strip the prefix for kind parity.
	if strings.TrimPrefix(kaa.Kind, "sealed ") != strings.TrimPrefix(fir.Kind, "sealed ") {
		t.Errorf("%s.%s: kind mismatch: kaa=%q fir=%q", file, fqn, kaa.Kind, fir.Kind)
	}
	// Supertype rendering legitimately differs between backends
	// (KAA emits `kotlin.Any`, FIR's renderReadable emits `Any`).
	// Assert each KAA supertype has a corresponding FIR entry whose
	// suffix matches — the strictest parity check we can do without
	// losing on benign renderer differences. Generic supertypes
	// (`Box<String>`) are compared on the head segment only, since
	// the type-arg list also differs by renderer (KAA may render
	// `kotlin.String` where FIR renders `String`).
	for _, want := range kaa.Supertypes {
		if !hasMatchingSupertype(want, fir.Supertypes) {
			t.Errorf("%s.%s: kaa supertype %q has no parity in fir %v", file, fqn, want, fir.Supertypes)
		}
	}
	// Class flags drive Go-side rule decisions (sealed/data/open
	// detection). A regression here would change rule behaviour
	// silently when the backend is switched.
	if kaa.IsSealed != fir.IsSealed {
		t.Errorf("%s.%s: isSealed mismatch: kaa=%v fir=%v", file, fqn, kaa.IsSealed, fir.IsSealed)
	}
	if kaa.IsData != fir.IsData {
		t.Errorf("%s.%s: isData mismatch: kaa=%v fir=%v", file, fqn, kaa.IsData, fir.IsData)
	}
	if kaa.IsAbstract != fir.IsAbstract {
		t.Errorf("%s.%s: isAbstract mismatch: kaa=%v fir=%v", file, fqn, kaa.IsAbstract, fir.IsAbstract)
	}
	// Type-parameter names must match exactly. The fixture uses
	// single-letter `T` so there's no naming ambiguity. Bounds
	// aren't on the wire yet, so the assertion is positional.
	if !slices.Equal(kaa.TypeParameters, fir.TypeParameters) {
		t.Errorf("%s.%s: typeParameters mismatch: kaa=%v fir=%v", file, fqn, kaa.TypeParameters, fir.TypeParameters)
	}
	// User-defined member names — FIR projects only declarations on
	// the class itself; KAA's set also includes inherited members
	// it surfaces by walking supertypes. So the parity contract is
	// `firUser ⊆ kaaUser`: every member FIR exposes must appear in
	// KAA. A FIR-side regression that drops a declared member fails
	// here; a KAA-side regression that drops an inherited member
	// doesn't, but the supertype-rendering check above guards the
	// inheritance edge that matters for rules.
	kaaUser := userMemberNames(kaa.Members)
	firUser := userMemberNames(fir.Members)
	for name := range firUser {
		if !kaaUser[name] {
			t.Errorf("%s.%s: fir member %q missing from kaa set %v", file, fqn, name, sortedSlice(kaaUser))
		}
	}
}

// hasMatchingSupertype compares on the last name segment after
// stripping generic type arguments, so `Box<String>` ↔ `parity.Box`
// and `Enum<Direction>` ↔ `kotlin.Enum` both match. The renderers
// disagree on FQN prefixes and on whether to include type-arg
// lists; rules only key off the tail.
func hasMatchingSupertype(want string, got []string) bool {
	wantTail := lastSegment(typeHead(want))
	for _, g := range got {
		if lastSegment(typeHead(g)) == wantTail {
			return true
		}
	}
	return false
}

func typeHead(s string) string {
	if i := strings.IndexByte(s, '<'); i >= 0 {
		return s[:i]
	}
	return s
}

// userMemberNames keeps only members whose names a rule would care
// about: user-defined functions and properties. Filtered out:
//   - Synthesized data-class members: `componentN`, `copy`, plus the
//     `equals`/`hashCode`/`toString` overrides and the `<init>`
//     constructor.
//   - Members inherited from `java.lang.Enum` that KAA surfaces and
//     FIR does not (`clone`, `finalize`, `ordinal`, `name`).
//   - Synthetic enum companion members FIR surfaces and KAA does
//     not (`values`, `valueOf`, `entries`).
//   - The enum entries themselves (uppercase names with no lower-
//     case letter), which FIR projects as members of the enum class
//     and KAA does not.
//
// What's left is the surface a rule reads: user-defined functions
// and properties on the declaration. That's the load-bearing parity
// contract; the rest is renderer drift.
func userMemberNames(members []*oracle.Member) map[string]bool {
	synth := map[string]bool{
		"equals": true, "hashCode": true, "toString": true, "copy": true, "<init>": true,
		"clone": true, "finalize": true, "ordinal": true, "name": true,
		"values": true, "valueOf": true, "entries": true,
	}
	out := make(map[string]bool, len(members))
	for _, m := range members {
		if synth[m.Name] {
			continue
		}
		if strings.HasPrefix(m.Name, "component") && len(m.Name) > len("component") {
			continue
		}
		if isEnumEntryName(m.Name) {
			continue
		}
		out[m.Name] = true
	}
	return out
}

// isEnumEntryName matches the Kotlin enum-entry convention: all
// uppercase letters / digits / underscores, with at least one
// uppercase letter. False positives on user-defined SHOUTY_CASE
// constants are acceptable for the parity gate because constants
// are also off the rule-decision path.
func isEnumEntryName(s string) bool {
	if s == "" {
		return false
	}
	hasUpper := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9', r == '_':
			// allowed
		default:
			return false
		}
	}
	return hasUpper
}

func sortedSlice(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func indexByBasename(files map[string]*oracle.File) map[string]*oracle.File {
	out := make(map[string]*oracle.File, len(files))
	for path, file := range files {
		out[filepath.Base(path)] = file
	}
	return out
}

func indexClassesByFqn(classes []*oracle.Class) map[string]*oracle.Class {
	out := make(map[string]*oracle.Class, len(classes))
	for _, cls := range classes {
		out[cls.FQN] = cls
	}
	return out
}

func sameKeys[T any](a, b map[string]T) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func lastSegment(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}
