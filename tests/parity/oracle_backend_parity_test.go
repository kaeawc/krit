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

// TestFirOracleDeclarationContract is the CI gate on the FIR type oracle's
// declaration wire shape. The one-shot CLI runs over a multi-file fixture
// covering the Kotlin shapes most likely to expose renderer drift (generics,
// sealed/enum/data, objects, type aliases, inline-reified, intersection
// bounds, plus the Kotlin 2.4 files), and every file's package, declared
// FQNs, class kinds and flags, supertype heads, type parameters and
// user-defined members are pinned to the values below. The expression
// surface is TestFirOracleFactContract's.
func TestFirOracleDeclarationContract(t *testing.T) {
	root := repoRoot(t)
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `make fir-jar` to enable the declaration contract")
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
	for name, body := range parityFixtureSources() {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// An explicit language mode, so the Kotlin 2.4 fixtures (named context
	// parameters, explicit backing fields, `@all:`) are not left to the
	// embedded compiler's defaults. The per-mode fact assertions live in
	// oracle_kotlin24_parity_test.go.
	opts := oracle.InvocationOptions{Classpath: []string{stdlib}, Kotlin: kotlin24Settings(t, kotlin24ModeNamed("language-2.4").args)}
	data := runOneShot(t, firJar, srcDir, filepath.Join(tmp, "fir.json"), opts)

	assertDeclarationContract(t, data)
}

// parityFixtureSources returns the source files that exercise the
// declaration shapes most likely to expose backend-rendering drift.
// Each file targets one axis so a regression points to a category,
// not a 200-line blob. The Kotlin 2.4 files (named context parameters,
// explicit backing fields, `@all:` and annotation default targets, #785)
// come from kotlin24FixtureSources.
func parityFixtureSources() map[string]string {
	sources := map[string]string{
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
		// Counter + companion intentionally omitted: FIR surfaces a
		// nested companion as a separate FQN, which is outside this
		// gate.
		"Objects.kt": `package parity

object Registry {
    fun register(id: String): Boolean = id.isNotEmpty()
}
`,
		// Inline-reified + type alias + intersection-bound use sites
		// are wrapped in classes because FIR projects a file
		// containing only top-level declarations with an empty
		// package, which is not what this fixture is meant to
		// exercise.
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
	for name, body := range kotlin24FixtureSources() {
		sources[name] = body
	}
	return sources
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

// TestOracleAcceptsClasspathFlag asserts that the krit-fir one-shot CLI
// accepts `--classpath` and analysis succeeds with the supplied JAR(s) on
// the classpath. Without this gate a typo would break
// `oracle.InvokeWithFilesWithOptions` for every project that ships an
// explicit classpath. The classpath supplied is kotlin-stdlib (the
// dependency every Kotlin source set needs anyway); FIR uses it directly to
// configure K2's compilation environment.
func TestOracleAcceptsClasspathFlag(t *testing.T) {
	root := repoRoot(t)
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir jar not found; run `make fir-jar`")
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

	opts := oracle.InvocationOptions{Classpath: []string{stdlib}}
	data := runOneShot(t, firJar, srcDir, filepath.Join(tmp, "fir.json"), opts)
	if data.Version != 1 {
		t.Errorf("version = %d, want 1", data.Version)
	}
	file := indexByBasename(data.Files)["Sample.kt"]
	if file == nil || file.Package != "x" {
		t.Fatalf("Sample.kt facts = %+v, want package x", file)
	}
}

// wantClass is the pinned declaration surface of one class. abstract is
// checked only when set.
type wantClass struct {
	kind       string
	sealed     bool
	data       bool
	abstract   *bool
	supertypes []string // last segment of each supertype head
	typeParams []string
	members    []string // userMemberNames, sorted
}

func boolPtr(b bool) *bool { return &b }

// wantDeclarations is the expected declaration surface per fixture file:
// the package and every declared class by FQN. The Kotlin 2.4 files pin
// their FQN set and package only; their members, types and annotations are
// asserted per language mode in oracle_kotlin24_parity_test.go.
func wantDeclarations() map[string]struct {
	pkg     string
	classes map[string]*wantClass
} {
	type file = struct {
		pkg     string
		classes map[string]*wantClass
	}
	class := func(supertype string, members ...string) *wantClass {
		return &wantClass{kind: "class", abstract: boolPtr(false), supertypes: []string{supertype}, members: members}
	}
	annotation := &wantClass{kind: "annotation", supertypes: []string{"Annotation"}}
	k24 := func(fqns ...string) map[string]*wantClass {
		out := map[string]*wantClass{}
		for _, fqn := range fqns {
			out[kotlin24Pkg+"."+fqn] = nil
		}
		return out
	}
	return map[string]file{
		"Basics.kt": {"parity", map[string]*wantClass{
			"parity.Greeter":     class("Any", "greet"),
			"parity.LoudGreeter": class("Greeter"),
		}},
		"Generics.kt": {"parity", map[string]*wantClass{
			"parity.Box":       {kind: "class", abstract: boolPtr(false), supertypes: []string{"Any"}, typeParams: []string{"T"}, members: []string{"unwrap", "value"}},
			"parity.StringBox": class("Box"),
		}},
		"Sealed.kt": {"parity", map[string]*wantClass{
			"parity.Shape":     {kind: "class", sealed: true, supertypes: []string{"Any"}, members: []string{"area"}},
			"parity.Circle":    {kind: "class", data: true, abstract: boolPtr(false), supertypes: []string{"Shape"}, members: []string{"area", "radius"}},
			"parity.Rectangle": {kind: "class", data: true, abstract: boolPtr(false), supertypes: []string{"Shape"}, members: []string{"area", "h", "w"}},
		}},
		"Enums.kt": {"parity", map[string]*wantClass{
			"parity.Direction": {kind: "enum", abstract: boolPtr(false), supertypes: []string{"Enum"}, members: []string{"describe"}},
		}},
		"Objects.kt": {"parity", map[string]*wantClass{
			"parity.Registry": {kind: "object", abstract: boolPtr(false), supertypes: []string{"Any"}, members: []string{"register"}},
		}},
		"Inline.kt": {"parity", map[string]*wantClass{
			"parity.TypeChecks": class("Any", "isA"),
		}},
		"Aliases.kt": {"parity", map[string]*wantClass{
			"parity.AliasUse": class("Any", "firstName"),
		}},
		// Where-clause bounds aren't on the wire; the function-level `T`
		// is not a class type parameter.
		"Intersection.kt": {"parity", map[string]*wantClass{
			"parity.Identity": class("Any", "identity"),
		}},
		"ContextParameters.kt": {kotlin24Pkg, func() map[string]*wantClass {
			out := k24("ContextParameters")
			out[kotlin24Marker] = annotation
			iface := func(member string) *wantClass {
				return &wantClass{kind: "interface", abstract: boolPtr(true), supertypes: []string{"Any"}, members: []string{member}}
			}
			out[kotlin24Pkg+".Logger"] = iface("log")
			out[kotlin24Pkg+".Clock"] = iface("now")
			return out
		}()},
		"SingleContextParameter.kt": {kotlin24Pkg, k24("SingleContextParameter")},
		"BackingFields.kt":          {kotlin24Pkg, k24("BackingFields")},
		"AnnotationTargets.kt": {kotlin24Pkg, func() map[string]*wantClass {
			out := k24("AnnotationTargets")
			for _, a := range []string{"OnBoth", "OnProperty", "OnField", "OnParam"} {
				out[kotlin24Pkg+"."+a] = annotation
			}
			return out
		}()},
		"AnnotationUse.kt": {kotlin24Pkg, k24("AnnotationUse")},
		"Lookalikes.kt":    {kotlin24Pkg, k24("Lookalikes")},
	}
}

func assertDeclarationContract(t *testing.T, data *oracle.Data) {
	t.Helper()
	if data.Version != 1 {
		t.Errorf("version = %d, want 1", data.Version)
	}
	if data.KotlinVersion == "" {
		t.Error("kotlinVersion is empty")
	}
	// Files are keyed by absolute path; compare by basename so the test
	// stays robust against `/var` ↔ `/private/var` drift on macOS.
	byBase := indexByBasename(data.Files)
	want := wantDeclarations()
	if got, wantKeys := sortedKeys(byBase), sortedKeys(want); !slices.Equal(got, wantKeys) {
		t.Fatalf("files = %v, want %v", got, wantKeys)
	}
	for name, wf := range want {
		f := byBase[name]
		if f.Package != wf.pkg {
			t.Errorf("%s: package = %q, want %q", name, f.Package, wf.pkg)
		}
		decls := indexClassesByFqn(f.Declarations)
		if got, wantKeys := sortedKeys(decls), sortedKeys(wf.classes); !slices.Equal(got, wantKeys) {
			t.Errorf("%s: declarations = %v, want %v", name, got, wantKeys)
			continue
		}
		for fqn, wc := range wf.classes {
			if wc != nil {
				assertClassContract(t, name, decls[fqn], wc)
			}
		}
	}
}

func assertClassContract(t *testing.T, file string, got *oracle.Class, want *wantClass) {
	t.Helper()
	where := file + ": " + got.FQN
	if got.Kind != want.kind {
		t.Errorf("%s: kind = %q, want %q", where, got.Kind, want.kind)
	}
	// Class flags drive Go-side rule decisions (sealed/data/open detection).
	if got.IsSealed != want.sealed || got.IsData != want.data {
		t.Errorf("%s: sealed=%v data=%v, want sealed=%v data=%v", where, got.IsSealed, got.IsData, want.sealed, want.data)
	}
	if want.abstract != nil && got.IsAbstract != *want.abstract {
		t.Errorf("%s: abstract = %v, want %v", where, got.IsAbstract, *want.abstract)
	}
	// Supertypes are compared on the last name segment of the head, so a
	// renderer change between `Box<String>` and `parity.Box<kotlin.String>`
	// does not fail the gate; rules key off the tail.
	var supers []string
	for _, s := range got.Supertypes {
		supers = append(supers, lastSegment(typeHead(s)))
	}
	sort.Strings(supers)
	if !slices.Equal(supers, want.supertypes) {
		t.Errorf("%s: supertypes = %v (%v), want %v", where, supers, got.Supertypes, want.supertypes)
	}
	if !slices.Equal(got.TypeParameters, want.typeParams) && (len(got.TypeParameters) != 0 || len(want.typeParams) != 0) {
		t.Errorf("%s: typeParameters = %v, want %v", where, got.TypeParameters, want.typeParams)
	}
	// A FIR-side regression that drops a declared member fails here.
	if members := sortedKeys(userMemberNames(got.Members)); !slices.Equal(members, want.members) && (len(members) != 0 || len(want.members) != 0) {
		t.Errorf("%s: members = %v, want %v", where, members, want.members)
	}
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
//   - Members inherited from `java.lang.Enum` (`clone`, `finalize`,
//     `ordinal`, `name`).
//   - Synthetic enum companion members (`values`, `valueOf`, `entries`).
//   - The enum entries themselves (uppercase names with no lower-
//     case letter), which FIR projects as members of the enum class.
//
// What's left is the surface a rule reads: user-defined functions
// and properties on the declaration. That's the load-bearing
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
// constants are acceptable for the contract gate because constants
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
