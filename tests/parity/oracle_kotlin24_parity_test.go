package parity_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
)

// Kotlin 2.4 syntax and semantics across the structural parser, FIR and KAA
// (#785). The fixtures cover the three constructs whose meaning the compiler
// decides and the pinned tree-sitter grammar cannot:
//
//   - named context parameters (declarations, use sites, shadowing),
//   - explicit backing fields (exposed property type vs the field's type),
//   - `@all:` and the annotation default target of a constructor `val`.
//
// They run through both oracle backends under explicit language modes, never
// the embedded compiler's defaults, using the same Kotlin settings plumbing
// the CLI uses (gradlemodel.ClassifyKotlinArgs -> InvocationOptions.Kotlin).
//
// Intentional FIR/KAA differences, also in docs/kotlin-syntax-support.md:
//
//   - KAA renders a generic return type without its arguments
//     (`kotlin.collections.List`), FIR with them
//     (`kotlin.collections.List<kotlin.Int>`). Rules key on the head.
//   - KAA records an expression fact only for resolved calls. FIR also records
//     the type of a plain reference, so the types of `logger` at a use site
//     (context parameter vs the local that shadows it) are asserted on FIR
//     only; KAA reports the same resolution through the inferred return type
//     of the enclosing function, which both assert.
//   - Neither backend puts a context parameter in a function's `params`.
//   - The oracle does not carry compiler errors, so a language-level feature
//     gate is observable there through the annotation default target only
//     (TestOracleKotlin24Modes); the FIR checker run reports the gates
//     themselves (TestFirKotlin24LanguageGates).
const (
	kotlin24Pkg       = "parity.kotlin24"
	kotlin24OnBoth    = kotlin24Pkg + ".OnBoth"
	kotlin24OnProp    = kotlin24Pkg + ".OnProperty"
	kotlin24Marker    = kotlin24Pkg + ".Marker"
	kotlin24Ctx       = kotlin24Pkg + ".ContextParameters"
	kotlin24Fields    = kotlin24Pkg + ".BackingFields"
	kotlin24Targets   = kotlin24Pkg + ".AnnotationTargets"
	kotlin24Lookalike = kotlin24Pkg + ".Lookalikes"
)

func kotlin24FixtureSources() map[string]string {
	return map[string]string{
		// Context parameters are 2.4 syntax. Several members with a named
		// context parameter in one class break the class in the pinned grammar:
		// the class keeps no body and its members are left at the top level of
		// the file (see assertKotlin24Structure and kotlin24_syntax_test.go).
		// The compilers see the full class.
		"ContextParameters.kt": `package parity.kotlin24

annotation class Marker

interface Logger {
    fun log(msg: String)
}

interface Clock {
    fun now(): Long
}

class ContextParameters {
    context(logger: Logger)
    fun record(msg: String) {
        logger.log(msg)
    }

    context(logger: Logger, clock: Clock)
    fun stamp(msg: String): Long {
        logger.log(msg)
        return clock.now()
    }

    context(_: Logger)
    fun anonymous(): Int = 1

    context(logger: Logger)
    val tagged: String
        get() = "tagged"

    @Marker
    context(logger: Logger)
    fun marked(): Int = 1

    context(logger: Logger)
    fun picksContext() = logger

    context(logger: Logger)
    fun shadowedByLocal() = run {
        val logger = 1
        logger
    }

    fun useSite(l: Logger, c: Clock): Long = with(l) { with(c) { stamp("x") } }

    fun callMarked(l: Logger): Int = with(l) { marked() }
}
`,
		// One named context parameter in a class parses with the class body
		// intact (plus a recovered, error-flagged property_declaration that
		// holds the context parameter list); see ContextParameters above for
		// what happens with several.
		"SingleContextParameter.kt": `package parity.kotlin24

class SingleContextParameter {
    context(logger: Logger)
    fun log(msg: String) {
        logger.log(msg)
    }

    fun plain(): Int = 1
}
`,
		// The exposed type of each property is the declared one; the explicit
		// backing field's type (MutableList, ArrayList) is not visible to users.
		"BackingFields.kt": `package parity.kotlin24

class BackingFields {
    val items: List<Int>
        field = mutableListOf<Int>()

    val names: Collection<String>
        field = ArrayList<String>()

    fun add(i: Int): Int = items.size
}
`,
		"AnnotationTargets.kt": `package parity.kotlin24

@Target(AnnotationTarget.PROPERTY, AnnotationTarget.VALUE_PARAMETER)
annotation class OnBoth

@Target(AnnotationTarget.PROPERTY)
annotation class OnProperty

@Target(AnnotationTarget.FIELD)
annotation class OnField

@Target(AnnotationTarget.VALUE_PARAMETER)
annotation class OnParam

class AnnotationTargets(
    @OnBoth val plain: Int,
    @all:OnBoth val everywhere: Int,
    @param:OnBoth val paramOnly: Int,
    @property:OnBoth val propertyOnly: Int,
    @field:OnField val fieldOnly: Int,
    @OnProperty val prop: Int,
    @OnParam val param: Int,
) {
    @OnBoth
    val body: Int = 0
}
`,
		"AnnotationUse.kt": `package parity.kotlin24

class AnnotationUse {
    fun read(a: AnnotationTargets): Int = a.plain + a.everywhere + a.propertyOnly + a.body

    fun backing(b: BackingFields): Int = b.items.size + b.names.size
}
`,
		// Negative lookalikes: nothing here is Kotlin 2.4 syntax. A property
		// named `field`, the implicit `field` of an accessor and a function
		// named `context` must not read as an explicit backing field or a
		// context parameter list.
		"Lookalikes.kt": `package parity.kotlin24

class Lookalikes {
    val first: Int = 1
    val field: Int = 2

    var counter: Int = 0
        set(value) {
            field = value
        }

    fun context(block: () -> Unit): Unit = block()

    fun useContext(): Unit = context { }
}
`,
	}
}

// kotlin24Mode is one explicit language mode. args are spelled as a Gradle
// build would, and go through the same allowlist the CLI applies.
type kotlin24Mode struct {
	name string
	args []string
	// plain is the expected property-level annotation of `@OnBoth val plain`
	// (a constructor `val` annotation with no use-site target, applicable to
	// both the parameter and the property): the 2.4 default applies it to both,
	// the older default to the first applicable target, the parameter.
	plain []string
}

// kotlin24ModernFlags enable, on a project pinned to 2.3, the three 2.4
// features the fixtures use.
var kotlin24ModernFlags = []string{"-Xcontext-parameters", "-Xexplicit-backing-fields", "-Xannotation-target-all"}

var kotlin24Modes = []kotlin24Mode{
	{name: "embedded-default", plain: []string{kotlin24OnBoth}},
	{name: "language-2.4", args: []string{"-language-version=2.4", "-api-version=2.4"}, plain: []string{kotlin24OnBoth}},
	{name: "language-2.4-first-only", args: []string{"-language-version=2.4", "-api-version=2.4", "-Xannotation-default-target=first-only"}},
	{name: "language-2.3-with-flags", args: append([]string{"-language-version=2.3", "-api-version=2.3"}, kotlin24ModernFlags...)},
	{name: "language-2.3-param-property", args: append(append([]string{"-language-version=2.3", "-api-version=2.3"}, kotlin24ModernFlags...), "-Xannotation-default-target=param-property"), plain: []string{kotlin24OnBoth}},
}

func kotlin24ModeNamed(name string) kotlin24Mode {
	for _, m := range kotlin24Modes {
		if m.name == name {
			return m
		}
	}
	panic("unknown Kotlin 2.4 mode " + name)
}

// kotlin24Settings classifies args exactly as the CLI does for a Gradle model,
// and fails when the allowlist drops any of them, so a mode never silently
// runs under fewer settings than it names.
func kotlin24Settings(t *testing.T, args []string) gradlemodel.KotlinSettings {
	t.Helper()
	ks := gradlemodel.ClassifyKotlinArgs(args)
	if len(ks.Rejected) > 0 {
		t.Fatalf("Kotlin arguments %v were not all applied: %v", args, ks.Rejected)
	}
	if len(args) > 0 && ks.IsZero() {
		t.Fatalf("Kotlin arguments %v produced no settings", args)
	}
	return ks
}

type kotlin24Backend struct {
	name    string
	jar     string
	backend oracle.Backend
}

func kotlin24Backends(t *testing.T) (backends []kotlin24Backend, stdlib string) {
	t.Helper()
	root := repoRoot(t)
	kaaJar := oracle.FindJar([]string{root})
	if kaaJar == "" {
		t.Skip("krit-types jar not found; run `make types-jar`")
	}
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `make fir-jar`")
	}
	stdlib = findKotlinStdlib()
	if stdlib == "" {
		t.Skip("kotlin-stdlib jar not found in Gradle cache; set KOTLIN_STDLIB_JAR")
	}
	return []kotlin24Backend{
		{name: "kaa", jar: kaaJar, backend: oracle.BackendKAA},
		{name: "fir", jar: firJar, backend: oracle.BackendFIR},
	}, stdlib
}

func writeKotlin24Sources(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range kotlin24FixtureSources() {
		writeFile(t, filepath.Join(dir, name), body)
	}
}

// TestOracleKotlin24Modes runs the Kotlin 2.4 fixtures through KAA and FIR
// under each explicit language mode and checks, per backend, the property and
// function annotations and types at the declaration and use sites; then that
// the two backends and the structural parser agree on the shared contract.
func TestOracleKotlin24Modes(t *testing.T) {
	backends, stdlib := kotlin24Backends(t)
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	writeKotlin24Sources(t, srcDir)
	structure := kotlin24Structure(t, srcDir)

	for _, mode := range kotlin24Modes {
		t.Run(mode.name, func(t *testing.T) {
			opts := oracle.InvocationOptions{Classpath: []string{stdlib}, Kotlin: kotlin24Settings(t, mode.args)}
			byBackend := map[string]*oracle.Data{}
			for _, b := range backends {
				data := runOneShot(t, b.jar, srcDir, filepath.Join(tmp, mode.name+"-"+b.name+".json"), opts)
				assertKotlin24Facts(t, b.name, data, mode.plain)
				assertKotlin24Structure(t, b.name, data, structure)
				byBackend[b.name] = data
			}
			assertKotlin24BackendContract(t, byBackend["kaa"], byBackend["fir"])
		})
	}
}

// TestOracleKotlin24RepeatedAnalyses runs the same sources through the cached
// oracle path three times per backend (cold through a daemon, served from the
// cache, then re-analyzed by the now warm daemon after an unrelated edit) and
// requires the annotation and type facts to stay identical and correct for the
// explicit language mode each time.
func TestOracleKotlin24RepeatedAnalyses(t *testing.T) {
	backends, stdlib := kotlin24Backends(t)
	for _, mode := range []kotlin24Mode{kotlin24ModeNamed("language-2.4"), kotlin24ModeNamed("language-2.3-with-flags")} {
		for _, b := range backends {
			t.Run(mode.name+"/"+b.name, func(t *testing.T) {
				t.Setenv("KRIT_DAEMON_CACHE", "on")
				t.Setenv(b.backend.JarEnvVar(), b.jar)
				kt := kotlin24Settings(t, mode.args)
				repo, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				src := filepath.Join(repo, "src")
				writeKotlin24Sources(t, src)
				unrelated := filepath.Join(src, "Unrelated.kt")
				writeFile(t, unrelated, "package parity.kotlin24.unrelated\n\nclass Unrelated\n")
				t.Cleanup(func() {
					if d, err := oracle.ConnectOrStartDaemon(b.jar, []string{src}, []string{stdlib}, false,
						oracle.WithBackend(b.backend), oracle.WithKotlinSettings(kt)); err == nil {
						_ = d.Shutdown()
					}
				})

				run := func(label string) (data *oracle.Data, hits, misses int) {
					t.Helper()
					tracker := perf.New(true)
					opts := oracle.InvocationOptions{Backend: b.backend, Classpath: []string{stdlib}, Kotlin: kt, Tracker: tracker}
					out := filepath.Join(repo, label+".json")
					if _, err := oracle.InvokeCachedWithOptions(b.jar, []string{src}, repo, out, "", false, nil, opts); err != nil {
						t.Fatalf("%s: %v", label, err)
					}
					entry, ok := wcFindTiming(tracker.GetTimings(), "cacheClassify")
					if !ok {
						t.Fatalf("%s: the oracle cache was bypassed", label)
					}
					return readOracleData(t, out), int(entry.Metrics["hits"]), int(entry.Metrics["misses"])
				}

				cold, hits, misses := run("cold")
				if hits != 0 || misses == 0 {
					t.Fatalf("cold run: %d hits, %d misses; want a full miss", hits, misses)
				}
				assertKotlin24Facts(t, b.name, cold, mode.plain)
				want := kotlin24Summary(cold)

				cached, hits, misses := run("cached")
				if misses != 0 || hits == 0 {
					t.Fatalf("second run: %d hits, %d misses; want it served from the cache", hits, misses)
				}
				assertKotlin24Facts(t, b.name, cached, mode.plain)
				assertSummaryEqual(t, "served from the cache", want, kotlin24Summary(cached))

				writeFile(t, unrelated, "package parity.kotlin24.unrelated\n\nclass Unrelated {\n    fun changed() = 1\n}\n")
				warm, _, misses := run("warm")
				if misses == 0 {
					t.Fatal("the unrelated edit was not re-analyzed; the warm daemon run proves nothing")
				}
				assertKotlin24Facts(t, b.name, warm, mode.plain)
				assertSummaryEqual(t, "re-analyzed by the warm daemon", want, kotlin24Summary(warm))
			})
		}
	}
}

func readOracleData(t *testing.T, path string) *oracle.Data {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var data oracle.Data
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return &data
}

func assertSummaryEqual(t *testing.T, what string, want, got []string) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("facts differ %s\nwant:\n  %s\ngot:\n  %s", what, strings.Join(want, "\n  "), strings.Join(got, "\n  "))
	}
}

// kotlin24Member finds a declared member of a class by name.
func kotlin24Member(data *oracle.Data, file, fqn, name string) *oracle.Member {
	f := indexByBasename(data.Files)[file]
	if f == nil {
		return nil
	}
	for _, cls := range f.Declarations {
		if cls.FQN != fqn {
			continue
		}
		for _, m := range cls.Members {
			if m.Name == name && m.Kind != "constructor" {
				return m
			}
		}
	}
	return nil
}

// kotlin24Line returns the 1-based line of the first fixture line containing
// needle (or equal to it when exact is set).
func kotlin24Line(t *testing.T, file, needle string, exact bool) int {
	t.Helper()
	for i, line := range strings.Split(kotlin24FixtureSources()[file], "\n") {
		if (exact && line == needle) || (!exact && strings.Contains(line, needle)) {
			return i + 1
		}
	}
	t.Fatalf("%s: no line containing %q", file, needle)
	return 0
}

func positionLine(pos string) int {
	l, _, _ := strings.Cut(pos, ":")
	n, err := strconv.Atoi(l)
	if err != nil {
		return -1
	}
	return n
}

// callFactsOnLine returns the resolved call targets on a source line with the
// annotations of the resolved symbol (an expression fact).
func callFactsOnLine(f *oracle.File, line int) map[string][]string {
	out := map[string][]string{}
	for pos, e := range f.Expressions {
		if positionLine(pos) == line && e.CallTarget != "" {
			ann := slices.Clone(e.Annotations)
			sort.Strings(ann)
			out[e.CallTarget] = ann
		}
	}
	return out
}

func annotationsOf(m *oracle.Member) []string {
	if m == nil || len(m.Annotations) == 0 {
		return nil
	}
	out := slices.Clone(m.Annotations)
	sort.Strings(out)
	return out
}

// assertKotlin24Facts pins the facts one backend reports for the fixtures
// under one language mode. plain is the expected property annotation of the
// constructor `val` that carries no use-site target.
func assertKotlin24Facts(t *testing.T, backend string, data *oracle.Data, plain []string) {
	t.Helper()
	if len(data.Files) == 0 {
		t.Fatalf("%s: no files in the oracle output", backend)
	}
	fail := func(format string, args ...any) {
		t.Helper()
		t.Errorf("%s: %s", backend, fmt.Sprintf(format, args...))
	}
	member := func(file, class, name string) *oracle.Member {
		t.Helper()
		m := kotlin24Member(data, file, class, name)
		if m == nil {
			fail("%s.%s is missing from %s", class, name, file)
		}
		return m
	}

	// Named context parameters: the declaration is a member with its own
	// kind and type; the context parameters are not value parameters.
	ctxFile := "ContextParameters.kt"
	for _, c := range []struct {
		name, kind, ret string
		params          []string
	}{
		{"record", "function", "kotlin.Unit", []string{"msg"}},
		{"stamp", "function", "kotlin.Long", []string{"msg"}},
		{"anonymous", "function", "kotlin.Int", nil},
		{"tagged", "property", "kotlin.String", nil},
		{"marked", "function", "kotlin.Int", nil},
		// Use-site resolution: an unshadowed `logger` is the context parameter
		// (Logger); a local `logger` shadows it (Int).
		{"picksContext", "function", kotlin24Pkg + ".Logger", nil},
		{"shadowedByLocal", "function", "kotlin.Int", nil},
		{"useSite", "function", "kotlin.Long", []string{"l", "c"}},
		{"callMarked", "function", "kotlin.Int", []string{"l"}},
	} {
		m := member(ctxFile, kotlin24Ctx, c.name)
		if m == nil {
			continue
		}
		if m.Kind != c.kind || typeHead(m.ReturnType) != c.ret {
			fail("%s: kind=%s type=%s, want %s %s", c.name, m.Kind, m.ReturnType, c.kind, c.ret)
		}
		var params []string
		for _, p := range m.Params {
			params = append(params, p.Name)
		}
		if !slices.Equal(params, c.params) {
			fail("%s: params=%v, want %v (context parameters are not value parameters)", c.name, params, c.params)
		}
	}
	if got := annotationsOf(member(ctxFile, kotlin24Ctx, "marked")); !slices.Equal(got, []string{kotlin24Marker}) {
		fail("marked: annotations=%v, want [%s] on the context function", got, kotlin24Marker)
	}
	if got := annotationsOf(member(ctxFile, kotlin24Ctx, "record")); got != nil {
		fail("record: annotations=%v, want none", got)
	}
	// ...and at the use site: the call resolves to the annotated context
	// function and carries its annotations.
	if f := indexByBasename(data.Files)[ctxFile]; f != nil {
		line := kotlin24Line(t, ctxFile, "with(l) { marked() }", false)
		got, ok := callFactsOnLine(f, line)[kotlin24Ctx+".marked"]
		if !ok || !slices.Equal(got, []string{kotlin24Marker}) {
			fail("call to marked() on line %d: annotations=%v (resolved=%v), want [%s]", line, got, ok, kotlin24Marker)
		}
		line = kotlin24Line(t, ctxFile, "with(l) { with(c) { stamp(\"x\") } }", false)
		if _, ok := callFactsOnLine(f, line)[kotlin24Ctx+".stamp"]; !ok {
			fail("call to stamp() on line %d does not resolve to the context function: %v", line, callFactsOnLine(f, line))
		}
		// FIR alone records the type of a plain reference (see the file comment).
		if backend == "fir" {
			assertReferenceType(t, f, kotlin24Line(t, ctxFile, "fun picksContext() = logger", false), kotlin24Pkg+".Logger", "the context parameter")
			assertReferenceType(t, f, kotlin24Line(t, ctxFile, "        logger", true), "kotlin.Int", "the local that shadows the context parameter")
		}
	}

	if m := member("SingleContextParameter.kt", kotlin24Pkg+".SingleContextParameter", "log"); m != nil &&
		(m.Kind != "function" || typeHead(m.ReturnType) != "kotlin.Unit" || len(m.Params) != 1 || m.Params[0].Name != "msg") {
		fail("SingleContextParameter.log: %+v, want a Unit function with the single value parameter msg", m)
	}

	// Explicit backing fields: the exposed type is the declared one. A backing
	// field type (MutableList, ArrayList) must not show through.
	for _, c := range []struct{ name, head, full string }{
		{"items", "kotlin.collections.List", "kotlin.collections.List<kotlin.Int>"},
		{"names", "kotlin.collections.Collection", "kotlin.collections.Collection<kotlin.String>"},
	} {
		m := member("BackingFields.kt", kotlin24Fields, c.name)
		if m == nil {
			continue
		}
		if m.Kind != "property" || typeHead(m.ReturnType) != c.head {
			fail("%s: kind=%s type=%s, want a property of %s", c.name, m.Kind, m.ReturnType, c.head)
		}
		if strings.Contains(m.ReturnType, "Mutable") || strings.Contains(m.ReturnType, "ArrayList") {
			fail("%s: type %s exposes the explicit backing field's type", c.name, m.ReturnType)
		}
		if backend == "fir" && m.ReturnType != c.full {
			fail("%s: type=%s, want %s", c.name, m.ReturnType, c.full)
		}
	}
	if m := kotlin24Member(data, "BackingFields.kt", kotlin24Fields, "field"); m != nil {
		fail("BackingFields has a member %q: an explicit backing field is not a property", m.Name)
	}
	if m := member("BackingFields.kt", kotlin24Fields, "add"); m != nil && typeHead(m.ReturnType) != "kotlin.Int" {
		fail("add: type=%s, want kotlin.Int", m.ReturnType)
	}
	// ...and at the use site, `b.items.size` resolves through the exposed type.
	if f := indexByBasename(data.Files)["AnnotationUse.kt"]; f != nil {
		if backend == "fir" {
			assertReferenceType(t, f, kotlin24Line(t, "AnnotationUse.kt", "b.items.size", false), "kotlin.Int", "items.size through the exposed type")
		}
		if m := member("AnnotationUse.kt", kotlin24Pkg+".AnnotationUse", "backing"); m != nil && typeHead(m.ReturnType) != "kotlin.Int" {
			fail("backing: type=%s, want kotlin.Int", m.ReturnType)
		}
	}

	// Annotation use-site targets on constructor properties.
	for _, c := range []struct {
		name string
		want []string
	}{
		{"plain", plain},
		{"everywhere", []string{kotlin24OnBoth}},
		{"paramOnly", nil},
		{"propertyOnly", []string{kotlin24OnBoth}},
		{"fieldOnly", nil},
		{"prop", []string{kotlin24OnProp}},
		{"param", nil},
		{"body", []string{kotlin24OnBoth}},
	} {
		m := member("AnnotationTargets.kt", kotlin24Targets, c.name)
		if m == nil {
			continue
		}
		if m.Kind != "property" || typeHead(m.ReturnType) != "kotlin.Int" {
			fail("%s: kind=%s type=%s, want a kotlin.Int property", c.name, m.Kind, m.ReturnType)
		}
		if got := annotationsOf(m); !slices.Equal(got, c.want) {
			fail("%s: property annotations=%v, want %v", c.name, got, c.want)
		}
	}

	// Lookalikes are plain members: a property named `field` and a function
	// named `context`.
	if m := member("Lookalikes.kt", kotlin24Lookalike, "field"); m != nil && m.Kind != "property" {
		fail("Lookalikes.field: kind=%s, want property", m.Kind)
	}
	if m := member("Lookalikes.kt", kotlin24Lookalike, "context"); m != nil && m.Kind != "function" {
		fail("Lookalikes.context: kind=%s, want function", m.Kind)
	}
}

func assertReferenceType(t *testing.T, f *oracle.File, line int, wantType, what string) {
	t.Helper()
	var seen []string
	for pos, e := range f.Expressions {
		if positionLine(pos) != line {
			continue
		}
		if e.Type == wantType {
			return
		}
		seen = append(seen, pos+"="+e.Type)
	}
	sort.Strings(seen)
	t.Errorf("fir: line %d has no expression of type %s (%s); has %v", line, wantType, what, seen)
}

// kotlin24Summary is the normalized, backend-independent view of the facts the
// fixtures assert, used to compare runs and backends: user members with type
// head, annotations and value parameter names, plus the resolved calls into
// the fixtures' own package by line.
func kotlin24Summary(data *oracle.Data) []string {
	var out []string
	for path, f := range data.Files {
		base := filepath.Base(path)
		if base == "Unrelated.kt" {
			continue // the file the repeated-analysis test edits
		}
		for _, cls := range f.Declarations {
			if !strings.HasPrefix(cls.FQN, kotlin24Pkg+".") {
				continue
			}
			names := userMemberNames(cls.Members)
			for _, m := range cls.Members {
				if !names[m.Name] || m.Kind == "constructor" {
					continue
				}
				var params []string
				for _, p := range m.Params {
					params = append(params, p.Name)
				}
				out = append(out, fmt.Sprintf("%s %s.%s %s %s ann=%v params=%v", base, cls.FQN, m.Name, m.Kind, typeHead(m.ReturnType), annotationsOf(m), params))
			}
		}
		seen := map[string]bool{}
		for pos, e := range f.Expressions {
			if !strings.HasPrefix(e.CallTarget, kotlin24Pkg+".") {
				continue
			}
			ann := slices.Clone(e.Annotations)
			sort.Strings(ann)
			seen[fmt.Sprintf("%s call line %d -> %s ann=%v", base, positionLine(pos), e.CallTarget, ann)] = true
		}
		for s := range seen {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// assertKotlin24BackendContract compares the shared fact contract of KAA and
// FIR. The renderers legitimately differ in type arguments (typeHead).
func assertKotlin24BackendContract(t *testing.T, kaa, fir *oracle.Data) {
	t.Helper()
	assertSummaryEqual(t, "between KAA and FIR", kotlin24Summary(kaa), kotlin24Summary(fir))
}

// kotlin24Structural is what the structural parser sees of one class.
type kotlin24Structural struct {
	file          string              // fixture file the class is declared in
	members       map[string]string   // member name -> function | property
	annotations   map[string][]string // class parameter name -> annotation text
	backingFields int                 // explicit_backing_field nodes
}

// kotlin24StructuralIndex is the structural view of every fixture: the classes
// by FQN and, per file, the names of the declarations the parser left at the
// top level of the file.
type kotlin24StructuralIndex struct {
	classes  map[string]*kotlin24Structural
	topLevel map[string]map[string]bool
}

// kotlin24Structure parses the fixtures with tree-sitter, as the structural
// rules see them, and indexes the members and annotations of each class.
func kotlin24Structure(t *testing.T, dir string) kotlin24StructuralIndex {
	t.Helper()
	out := kotlin24StructuralIndex{classes: map[string]*kotlin24Structural{}, topLevel: map[string]map[string]bool{}}
	for name := range kotlin24FixtureSources() {
		f, err := scanner.ParseFile(context.Background(), filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		out.topLevel[name] = map[string]bool{}
		f.FlatForEachChild(0, func(decl uint32) {
			switch f.FlatType(decl) {
			case "function_declaration":
				out.topLevel[name][f.FlatChildTextOrEmpty(decl, "simple_identifier")] = true
			case "property_declaration":
				if v, ok := f.FlatFindChild(decl, "variable_declaration"); ok {
					out.topLevel[name][f.FlatChildTextOrEmpty(v, "simple_identifier")] = true
				}
			}
		})
		f.FlatWalkNodes(0, "class_declaration", func(cls uint32) {
			className := f.FlatChildTextOrEmpty(cls, "type_identifier")
			s := &kotlin24Structural{file: name, members: map[string]string{}, annotations: map[string][]string{}}
			out.classes[kotlin24Pkg+"."+className] = s
			if params, ok := f.FlatFindChild(cls, "primary_constructor"); ok {
				f.FlatForEachChild(params, func(p uint32) {
					if f.FlatType(p) != "class_parameter" || !f.FlatHasChildOfType(p, "binding_pattern_kind") {
						return
					}
					name := f.FlatChildTextOrEmpty(p, "simple_identifier")
					s.members[name] = "property"
					if mods, ok := f.FlatFindChild(p, "modifiers"); ok {
						f.FlatForEachChild(mods, func(a uint32) {
							if f.FlatType(a) == "annotation" {
								s.annotations[name] = append(s.annotations[name], f.FlatNodeText(a))
							}
						})
					}
				})
			}
			body, ok := f.FlatFindChild(cls, "class_body")
			if !ok {
				return
			}
			f.FlatForEachChild(body, func(m uint32) {
				switch f.FlatType(m) {
				case "function_declaration":
					s.members[f.FlatChildTextOrEmpty(m, "simple_identifier")] = "function"
				case "property_declaration":
					if decl, ok := f.FlatFindChild(m, "variable_declaration"); ok {
						s.members[f.FlatChildTextOrEmpty(decl, "simple_identifier")] = "property"
					}
				case scanner.ExplicitBackingFieldNodeType:
					s.backingFields++
				}
			})
		})
	}
	return out
}

// assertKotlin24Structure compares the structural parser's view of each class
// with a backend's: the same declared members, no property named after an
// explicit backing field, and the same count of explicit backing fields.
func assertKotlin24Structure(t *testing.T, backend string, data *oracle.Data, structure kotlin24StructuralIndex) {
	t.Helper()
	oracleMembers := map[string]map[string]string{}
	for _, f := range data.Files {
		for _, cls := range f.Declarations {
			names := userMemberNames(cls.Members)
			ms := map[string]string{}
			for _, m := range cls.Members {
				if names[m.Name] && m.Kind != "constructor" {
					ms[m.Name] = m.Kind
				}
			}
			oracleMembers[cls.FQN] = ms
		}
	}
	for fqn, s := range structure.classes {
		got, ok := oracleMembers[fqn]
		if !ok {
			t.Errorf("%s: class %s is in the source but not in the oracle output", backend, fqn)
			continue
		}
		if fqn == kotlin24Ctx {
			// KNOWN LIMITATION of the pinned grammar (docs/kotlin-syntax-support.md,
			// kotlin24_syntax_test.go): two or more members with a named context
			// parameter in one class break the class body. The class keeps no
			// members and every one of them is left at the top level of the file,
			// where class-scoped rules cannot attribute it. Both backends see the
			// full class. Only the names survive, as file-level declarations.
			if len(s.members) != 0 {
				t.Errorf("%s: the structural parser now keeps %d members in %s; the grammar limitation is fixed, so assert the members like any other class and update the docs", backend, len(s.members), fqn)
			}
			for name := range got {
				if !structure.topLevel[s.file][name] {
					t.Errorf("%s: member %s of %s is not even a file-level declaration in the structural view", backend, name, fqn)
				}
			}
			continue
		}
		if !reflect.DeepEqual(got, s.members) {
			t.Errorf("%s: %s members differ from the structural parser's\noracle:     %v\nstructural: %v", backend, fqn, got, s.members)
		}
		// Two explicit backing fields in BackingFields; none anywhere else, in
		// particular not for the accessor `field` or the property named `field`.
		want := 0
		if fqn == kotlin24Fields {
			want = 2
		}
		if s.backingFields != want {
			t.Errorf("%s: structural parser sees %d explicit backing fields in %s, want %d", backend, s.backingFields, fqn, want)
		}
	}
	// Structurally the annotation is whatever is written, in every language
	// mode; where it lands is the compiler's to decide. `@all:` is the one
	// target the pinned grammar cannot parse: it keeps `@all` (and an ERROR
	// span for `:OnBoth`), so the real annotation is invisible to structural
	// rules (docs/kotlin-syntax-support.md).
	wantAnn := map[string][]string{
		"plain": {"@OnBoth"}, "everywhere": {"@all"}, "paramOnly": {"@param:OnBoth"},
		"propertyOnly": {"@property:OnBoth"}, "fieldOnly": {"@field:OnField"},
		"prop": {"@OnProperty"}, "param": {"@OnParam"},
	}
	if got := structure.classes[kotlin24Targets].annotations; !reflect.DeepEqual(got, wantAnn) {
		t.Errorf("%s: structural annotations on AnnotationTargets = %v, want %v", backend, got, wantAnn)
	}
}
