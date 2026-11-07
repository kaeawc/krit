package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const ebfRule = "ExplicitBackingFieldCandidate"

const ebfFlowImports = `package test

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.update

`

func ebfKotlinProfile(languageVersion, compilerVersion string) librarymodel.ProjectProfile {
	var p librarymodel.ProjectProfile
	if languageVersion != "" {
		p.Kotlin.LanguageVersion = librarymodel.KnownVersion(languageVersion)
	}
	if compilerVersion != "" {
		p.Kotlin.Compiler = librarymodel.KnownVersion(compilerVersion)
	}
	return p
}

// runEbf runs the rule with the given project Kotlin profile.
func runEbf(t *testing.T, src string, profile librarymodel.ProjectProfile) []scanner.Finding {
	t.Helper()
	file := parseInline(t, src)
	for _, r := range api.Registry {
		if r.ID == ebfRule {
			d := rules.NewDispatcher([]*api.Rule{r}, nil)
			d.SetLibraryFacts(librarymodel.FactsForProfile(profile))
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %s not registered", ebfRule)
	return nil
}

func runEbfKnown(t *testing.T, src string) []scanner.Finding {
	return runEbf(t, src, ebfKotlinProfile("2.4", ""))
}

func TestExplicitBackingFieldCandidate_Shapes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"getter expression", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n}\n", 1},
		{"getter block", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int>\n        get() { return _s }\n}\n", 1},
		{"asStateFlow initializer", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> = _s.asStateFlow()\n}\n", 1},
		{"asStateFlow inferred type", "class A {\n    private val _s = MutableStateFlow(0)\n    val s = _s.asStateFlow()\n}\n", 1},
		{"asSharedFlow getter", "class A {\n    private val _s = MutableSharedFlow<Int>()\n    val s: SharedFlow<Int> get() = _s.asSharedFlow()\n}\n", 1},
		{"state flow as shared flow", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: SharedFlow<Int> get() = _s\n}\n", 1},
		{"declared backing type", "class A {\n    private val _s: MutableStateFlow<Int?> = MutableStateFlow(null)\n    val s: StateFlow<Int?> get() = _s\n}\n", 1},
		{"public after, exposed first", "class A {\n    val s: StateFlow<Int> get() = _s\n    private val _s = MutableStateFlow(0)\n}\n", 1},
		{"internal visibility", "class A {\n    private val _s = MutableStateFlow(0)\n    internal val s: StateFlow<Int> get() = _s\n}\n", 1},
		{"object member", "object A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n}\n", 1},
		{"top level", "private val _s = MutableStateFlow(0)\nval s: StateFlow<Int> get() = _s\n", 1},
		{"this qualified", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = this._s\n}\n", 1},
		{"override in final class", "class A : Holder {\n    private val _s = MutableStateFlow(0)\n    override val s: StateFlow<Int> get() = _s\n}\ninterface Holder { val s: StateFlow<Int> }\n", 1},
		{"unrelated name", "class A {\n    private val backing = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = backing\n}\n", 1},

		{"not private", "class A {\n    internal val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n}\n", 0},
		{"var backing", "class A {\n    private var _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n}\n", 0},
		{"annotated backing", "class A {\n    @Volatile private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n}\n", 0},
		{"public type is the mutable type", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: MutableStateFlow<Int> get() = _s\n}\n", 0},
		{"inferred identity getter", "class A {\n    private val _s = MutableStateFlow(0)\n    val s get() = _s\n}\n", 0},
		{"private exposing property", "class A {\n    private val _s = MutableStateFlow(0)\n    private val s: StateFlow<Int> get() = _s\n}\n", 0},
		{"custom getter logic", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s.also { println(it) }\n}\n", 0},
		{"getter reads another property", "class A {\n    private val _s = MutableStateFlow(0)\n    private val _t = Foo()\n    val s: StateFlow<Int> get() = _t\n}\n", 0},
		{"two exposures", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n    val t: StateFlow<Int> get() = _s\n}\n", 0},
		{"open property", "open class A {\n    private val _s = MutableStateFlow(0)\n    open val s: StateFlow<Int> get() = _s\n}\n", 0},
		{"override in open class", "open class A : Holder {\n    private val _s = MutableStateFlow(0)\n    override val s: StateFlow<Int> get() = _s\n}\ninterface Holder { val s: StateFlow<Int> }\n", 0},
		{"var exposing property", "class A {\n    private val _s = MutableStateFlow(0)\n    var s: StateFlow<Int> = _s.asStateFlow()\n}\n", 0},
		{"delegated exposing property", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> by lazy { _s }\n}\n", 0},
		{"jvmfield exposing property", "class A {\n    private val _s = MutableStateFlow(0)\n    @JvmField val s: StateFlow<Int> = _s\n}\n", 0},
		{"extension exposing property", "class A {\n    private val _s = MutableStateFlow(0)\n    val Int.s: StateFlow<Int> get() = _s\n}\n", 0},
		{"toList copies", "class A {\n    private val _l = mutableListOf(1)\n    val l: List<Int> get() = _l.toList()\n}\n", 0},
		{"wrong wrapper", "class A {\n    private val _s = MutableSharedFlow<Int>()\n    val s: StateFlow<Int> = _s.asStateFlow()\n}\n", 0},
		{"unknown backing type", "class A {\n    private val _s = Foo()\n    val s: Bar get() = _s\n}\n", 0},
		{"existing explicit backing field", "class A {\n    val s: StateFlow<Int>\n        field = MutableStateFlow(0)\n}\n", 0},
		{"interface", "interface A {\n    val s: StateFlow<Int>\n}\n", 0},
		{"lookalike without import", "package test\n\nclass A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n}\n", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.body
			if !strings.HasPrefix(src, "package") {
				src = ebfFlowImports + src
			}
			if got := len(runEbfKnown(t, src)); got != tc.want {
				t.Fatalf("findings = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestExplicitBackingFieldCandidate_Families(t *testing.T) {
	cases := []struct {
		name, imports, body string
		want                int
	}{
		{"live data", "import androidx.lifecycle.MutableLiveData\nimport androidx.lifecycle.LiveData\n",
			"class A {\n    private val _c = MutableLiveData<String>()\n    val c: LiveData<String> get() = _c\n}\n", 1},
		{"mutable list", "", "class A {\n    private val _l = mutableListOf<Int>()\n    val l: List<Int> get() = _l\n}\n", 1},
		{"mutable set declared", "", "class A {\n    private val _l: MutableSet<Int> = mutableSetOf()\n    val l: Set<Int> get() = _l\n}\n", 1},
		{"mutable map", "", "class A {\n    private val _m = mutableMapOf<Int, Int>()\n    val m: Map<Int, Int> get() = _m\n}\n", 1},
		{"list as collection", "", "class A {\n    private val _l = ArrayList<Int>()\n    val l: Collection<Int> get() = _l\n}\n", 1},
		{"compose state", "import androidx.compose.runtime.mutableStateOf\nimport androidx.compose.runtime.State\n",
			"class A {\n    private val _x = mutableStateOf(0)\n    val x: State<Int> get() = _x\n}\n", 1},
		{"channel", "import kotlinx.coroutines.channels.Channel\nimport kotlinx.coroutines.channels.ReceiveChannel\n",
			"class A {\n    private val _c = Channel<Int>()\n    val c: ReceiveChannel<Int> get() = _c\n}\n", 1},
		{"list exposed as set", "", "class A {\n    private val _l = mutableListOf<Int>()\n    val l: Set<Int> get() = _l\n}\n", 0},
		{"live data without import", "", "class A {\n    private val _c = MutableLiveData<String>()\n    val c: LiveData<String> get() = _c\n}\n", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test\n\n" + tc.imports + "\n" + tc.body
			if got := len(runEbfKnown(t, src)); got != tc.want {
				t.Fatalf("findings = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestExplicitBackingFieldCandidate_ReportsOnBackingProperty(t *testing.T) {
	src := ebfFlowImports + "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n}\n"
	findings := runEbfKnown(t, src)
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if want := strings.Count(ebfFlowImports, "\n") + 2; findings[0].Line != want {
		t.Fatalf("line = %d, want %d (the backing property)", findings[0].Line, want)
	}
	if !strings.Contains(findings[0].Message, "'_s'") || !strings.Contains(findings[0].Message, "'s'") {
		t.Fatalf("message should name both properties: %q", findings[0].Message)
	}
}

func TestExplicitBackingFieldCandidate_LanguageVersionGate(t *testing.T) {
	src := ebfFlowImports + "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n}\n"
	cases := []struct {
		name     string
		profile  librarymodel.ProjectProfile
		want     int
		wantConf float64
	}{
		{"language 2.3 skips", ebfKotlinProfile("2.3", ""), 0, 0},
		{"language 2.4 reports", ebfKotlinProfile("2.4", ""), 1, api.ConfidenceMedium},
		{"language 2.5 reports", ebfKotlinProfile("2.5", ""), 1, api.ConfidenceMedium},
		{"language beats a newer compiler", ebfKotlinProfile("2.3", "2.4.21"), 0, 0},
		{"older language, compiler 1.9", ebfKotlinProfile("", "1.9.24"), 0, 0},
		{"compiler 2.4.21 reports", ebfKotlinProfile("", "2.4.21"), 1, api.ConfidenceMedium},
		{"compiler 2.2 skips", ebfKotlinProfile("", "2.2.20"), 0, 0},
		{"unknown version reports with low confidence", ebfKotlinProfile("", ""), 1, api.ConfidenceLow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runEbf(t, src, tc.profile)
			if len(findings) != tc.want {
				t.Fatalf("findings = %d, want %d", len(findings), tc.want)
			}
			if tc.want == 1 && findings[0].Confidence != tc.wantConf {
				t.Fatalf("confidence = %v, want %v", findings[0].Confidence, tc.wantConf)
			}
		})
	}
}

// applyEbfFix applies the single finding's fix and checks the result parses with exactly wantFields explicit backing fields.
func applyEbfFix(t *testing.T, src string, wantFields int) string {
	t.Helper()
	findings := runEbfKnown(t, src)
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	fixed := applySingleFix(t, src, findings[0])
	path := filepath.Join(t.TempDir(), "fixed.kt")
	if err := os.WriteFile(path, []byte(fixed), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	got := 0
	file.FlatWalkNodes(0, scanner.ExplicitBackingFieldNodeType, func(uint32) { got++ })
	if got != wantFields {
		t.Fatalf("fixed source has %d explicit backing fields, want %d:\n%s", got, wantFields, fixed)
	}
	if again := runEbfKnown(t, fixed); len(again) != 0 {
		t.Fatalf("fixed source still reports %d findings:\n%s", len(again), fixed)
	}
	return fixed
}

func TestExplicitBackingFieldCandidate_Fix(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{
			"getter, uses renamed",
			"class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n\n    fun bump() {\n        _s.value = 1\n        _s.update { it + 1 }\n    }\n}\n",
			"class A {\n    val s: StateFlow<Int>\n        field = MutableStateFlow(0)\n\n    fun bump() {\n        s.value = 1\n        s.update { it + 1 }\n    }\n}\n",
		},
		{
			"asStateFlow initializer, exposed property first",
			"class A {\n    val s: StateFlow<Int> = _s.asStateFlow()\n    private val _s = MutableStateFlow(0)\n\n    fun bump() {\n        _s.tryEmit(1)\n    }\n}\n",
			"class A {\n    val s: StateFlow<Int>\n        field = MutableStateFlow(0)\n\n    fun bump() {\n        s.tryEmit(1)\n    }\n}\n",
		},
		{
			"declared backing type is kept",
			"class A {\n    private val _s: MutableStateFlow<Int?> = MutableStateFlow(null)\n    val s: StateFlow<Int?> get() = _s\n}\n",
			"class A {\n    val s: StateFlow<Int?>\n        field: MutableStateFlow<Int?> = MutableStateFlow(null)\n}\n",
		},
		{
			"sibling getter on its own line",
			"class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int>\n        get() = _s.asStateFlow()\n}\n",
			"class A {\n    val s: StateFlow<Int>\n        field = MutableStateFlow(0)\n}\n",
		},
		{
			"modifiers and annotations of the public property stay",
			"class A {\n    private val _s = MutableStateFlow(0)\n    @Suppress(\"x\") internal val s: StateFlow<Int> get() = _s\n}\n",
			"class A {\n    @Suppress(\"x\") internal val s: StateFlow<Int>\n        field = MutableStateFlow(0)\n}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := ebfFlowImports + tc.body
			fixed := applyEbfFix(t, src, 1)
			if want := ebfFlowImports + tc.want; fixed != want {
				t.Fatalf("fixed source:\n%s\nwant:\n%s", fixed, want)
			}
		})
	}
}

func TestExplicitBackingFieldCandidate_NoFixWhenUnsafe(t *testing.T) {
	cases := []struct{ name, body string }{
		{"passed as an argument", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n    fun f() = register(_s)\n}\n"},
		{"read as a plain value", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n    fun f() { val x = _s }\n}\n"},
		{"this qualified use", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n    fun f() { this._s.value = 1 }\n}\n"},
		{"string template use", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n    fun f() = \"$_s\"\n}\n"},
		{"shadowing parameter", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n    fun f(s: Int) { _s.value = s }\n}\n"},
		{"shadowing lambda parameter", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n    fun f() { listOf(1).forEach { s -> _s.value = s } }\n}\n"},
		{"members not adjacent", "class A {\n    private val _s = MutableStateFlow(0)\n    val other = 1\n    val s: StateFlow<Int> get() = _s\n}\n"},
		{"comment on the backing property", "class A {\n    // Mutable side.\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n}\n"},
		{"inferred public type", "class A {\n    private val _s = MutableStateFlow(0)\n    val s = _s.asStateFlow()\n}\n"},
		{"top level", "private val _s = MutableStateFlow(0)\nval s: StateFlow<Int> get() = _s\n"},
		{"named argument spelled like the backing property", "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n    fun f() = call(_s = 1)\n}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runEbfKnown(t, ebfFlowImports+tc.body)
			if len(findings) != 1 {
				t.Fatalf("findings = %d, want 1", len(findings))
			}
			if findings[0].Fix != nil {
				t.Fatalf("expected report-only, got fix %q", findings[0].Fix.Replacement)
			}
		})
	}
}

func TestExplicitBackingFieldCandidate_CommentsMentioningTheBackingNameDoNotBlockTheFix(t *testing.T) {
	src := ebfFlowImports + "class A {\n    private val _s = MutableStateFlow(0)\n    val s: StateFlow<Int> get() = _s\n\n    // Writes go through _s.\n    fun bump() {\n        _s.value = 1\n    }\n}\n"
	fixed := applyEbfFix(t, src, 1)
	if !strings.Contains(fixed, "        s.value = 1\n") || !strings.Contains(fixed, "// Writes go through _s.") {
		t.Fatalf("unexpected fixed source:\n%s", fixed)
	}
}

// A property with an explicit backing field exposes its declared type, so the
// leak rule must read that type and not the mutable field type.
func TestStateFlowMutableLeak_ExplicitBackingFields(t *testing.T) {
	forms := map[string]string{
		"multi-line inferred": "val s: StateFlow<Int>\n        field = MutableStateFlow(0)",
		"multi-line typed":    "val s: StateFlow<Int>\n        field: MutableStateFlow<Int> = MutableStateFlow(0)",
		"single-line":         "val s: StateFlow<Int> field = MutableStateFlow(0)",
		"single-line typed":   "val s: StateFlow<Int> field: MutableStateFlow<Int> = MutableStateFlow(0)",
		"single-line typed after a recovered field": "val a: SharedFlow<Int>\n        field = MutableSharedFlow<Int>()\n\n    val s: StateFlow<Int> field: MutableStateFlow<Int> = MutableStateFlow(0)",
		"shared flow":       "val s: SharedFlow<Int> field = MutableSharedFlow()",
		"shared flow typed": "val s: SharedFlow<Int> field: MutableSharedFlow<Int> = MutableSharedFlow()",
	}
	for name, form := range forms {
		t.Run(name, func(t *testing.T) {
			src := ebfFlowImports + "class A {\n    " + form + "\n}\n"
			if got := runRuleByName(t, "StateFlowMutableLeak", src); len(got) != 0 {
				t.Fatalf("explicit backing field flagged as a leak: %v", got)
			}
		})
	}
	leaked := ebfFlowImports + "class A {\n    val s: MutableStateFlow<Int>\n        field = MutableStateFlow(0)\n}\n"
	if got := runRuleByName(t, "StateFlowMutableLeak", leaked); len(got) != 1 {
		t.Fatalf("declared MutableStateFlow type must still be reported, got %d findings", len(got))
	}
}

func TestExplicitBackingFieldCandidate_FixFollowsTheFilesIndentation(t *testing.T) {
	src := ebfFlowImports + "class A {\n  private val _s = MutableStateFlow(0)\n  val s: StateFlow<Int> get() = _s\n}\n"
	want := ebfFlowImports + "class A {\n  val s: StateFlow<Int>\n    field = MutableStateFlow(0)\n}\n"
	if fixed := applyEbfFix(t, src, 1); fixed != want {
		t.Fatalf("fixed source:\n%s\nwant:\n%s", fixed, want)
	}
}
