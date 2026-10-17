package scanner

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFindingsBundleStore_RoundTrip(t *testing.T) {
	root := t.TempDir()
	fp := RunFingerprint{
		Version:      "v1",
		Rules:        "rules",
		Config:       "config",
		SourceSet:    "sources",
		CrossFile:    "cross",
		Android:      "android",
		LibraryFacts: "libs",
	}
	cols := CollectFindings([]Finding{{
		File:     "/repo/A.kt",
		Line:     1,
		Col:      2,
		Rule:     "RuleA",
		Severity: "warning",
		Message:  "message",
	}})
	store := DiskFindingsBundleStore{}
	if err := store.Save(root, fp, &cols); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok := store.Load(root, fp)
	if !ok {
		t.Fatal("Load returned ok=false")
	}
	if !reflect.DeepEqual(got.Findings(), cols.Findings()) {
		t.Fatalf("findings = %#v, want %#v", got.Findings(), cols.Findings())
	}
}

func TestFindingsBundleStore_FingerprintChangeMisses(t *testing.T) {
	root := t.TempDir()
	store := DiskFindingsBundleStore{}
	fp := RunFingerprint{Version: "v1", Rules: "rules"}
	cols := CollectFindings([]Finding{{File: "/repo/A.kt", Rule: "RuleA"}})
	if err := store.Save(root, fp, &cols); err != nil {
		t.Fatalf("Save: %v", err)
	}
	fp.Rules = "other-rules"
	if _, ok := store.Load(root, fp); ok {
		t.Fatal("Load returned hit after fingerprint changed")
	}
}

func TestConservativeDeltaPlanner_AllowsSingleFileWhenGlobalInputsStable(t *testing.T) {
	prev := RunFingerprint{Version: "v1", Rules: "rules", Config: "config", CrossFile: "cross", Android: "android", LibraryFacts: "libs"}
	cur := prev
	cur.SourceSet = "sources-after-edit"
	plan := (ConservativeDeltaPlanner{}).Plan(prev, cur, []string{"/repo/A.kt"})
	if !plan.ReusePrevious {
		t.Fatal("expected reusable delta plan")
	}
	if !reflect.DeepEqual(plan.AffectedPaths, []string{"/repo/A.kt"}) {
		t.Fatalf("AffectedPaths = %v", plan.AffectedPaths)
	}
}

func TestConservativeDeltaPlanner_GlobalFingerprintChangeDisablesDelta(t *testing.T) {
	prev := RunFingerprint{Version: "v1", Rules: "rules", Config: "config"}
	cur := prev
	cur.Config = "changed"
	plan := (ConservativeDeltaPlanner{}).Plan(prev, cur, []string{"/repo/A.kt"})
	if plan.ReusePrevious {
		t.Fatal("expected config change to disable delta")
	}
}

// Kotlin compiler settings are part of the findings identity: the same
// sources under another language level may be analyzed differently.
func TestFindingsBundleKey_CompilerSettingsOnlyChange(t *testing.T) {
	base := RunFingerprint{Version: "v1", Rules: "rules", Config: "config", SourceSet: "src", CrossFile: "cross"}
	withOld := base
	withOld.CompilerSettings = "kotlin-settings-v1|lang=1.9"
	withNew := base
	withNew.CompilerSettings = "kotlin-settings-v1|lang=2.4"
	keys := map[string]string{
		"none": FindingsBundleKey(base), "old": FindingsBundleKey(withOld), "new": FindingsBundleKey(withNew),
	}
	if keys["none"] == keys["old"] || keys["old"] == keys["new"] || keys["none"] == keys["new"] {
		t.Fatalf("settings-only change kept the bundle key: %v", keys)
	}
	// An absent setting hashes exactly as before the field existed.
	legacy := RunFingerprint{Version: "v1", Rules: "rules", Config: "config", SourceSet: "src", CrossFile: "cross", CompilerSettings: ""}
	if FindingsBundleKey(legacy) != FindingsBundleKey(base) {
		t.Fatal("empty CompilerSettings changed the key")
	}
	plan := (ConservativeDeltaPlanner{}).Plan(withOld, withNew, []string{"/repo/A.kt"})
	if plan.ReusePrevious {
		t.Fatal("a compiler settings change must not be treated as a single-file delta")
	}
	if !(ConservativeDeltaPlanner{}).Plan(withOld, withOld, []string{"/repo/A.kt"}).ReusePrevious {
		t.Fatal("unchanged settings must still allow a single-file delta")
	}
}

func TestApplyDelta_ReplacesAffectedRowsAndKeepsProjectRows(t *testing.T) {
	prev := CollectFindings([]Finding{
		{File: "/repo/A.kt", Line: 2, Rule: "RuleA", Message: "old"},
		{File: "/repo/B.kt", Line: 1, Rule: "RuleA", Message: "keep"},
		{File: "", Line: 1, Rule: "ProjectRule", Message: "project"},
	})
	replacement := CollectFindings([]Finding{
		{File: "/repo/A.kt", Line: 1, Rule: "RuleA", Message: "new"},
	})
	got := ApplyDelta(&prev, &replacement, []string{"/repo/A.kt"})
	messages := make([]string, 0, got.Len())
	for i := 0; i < got.Len(); i++ {
		messages = append(messages, got.MessageAt(i))
	}
	want := []string{"project", "new", "keep"}
	if !reflect.DeepEqual(messages, want) {
		t.Fatalf("messages = %v, want %v", messages, want)
	}
}

func TestFileStructuralFingerprint_PropertyDeclaredType(t *testing.T) {
	fp := func(src string) string {
		return FileStructuralFingerprint(parseKotlinSnippet(t, src))
	}
	nullable := fp("class R { val name: String? = null }")
	str := fp("class R { val name: String = \"\" }")
	num := fp("class R { val name: Int = 0 }")
	if nullable == str || str == num || nullable == num {
		t.Fatalf("declared type/nullability must move the fingerprint: %s %s %s", nullable, str, num)
	}
	if fp("class R { val name: Map<String, Int?>? = null }") == fp("class R { val name: Map<String, Int>? = null }") {
		t.Fatal("generic argument nullability must move the fingerprint")
	}
	if fp("class R { val name:   String = \"a\" }") != fp("class R { val name: String = \"b\" }") {
		t.Fatal("whitespace and initializer-only edits with an explicit type must be stable")
	}
	// Cases below share simple names so reference names cannot mask them.
	if fp("val Foo.bar: List<Int> get() = 1") == fp("val Foo.bar: List<Int?> get() = 1") {
		t.Fatal("extension property type nullability must move the fingerprint")
	}
	if fp("val Foo.bar: Int get() = 1") == fp("val Foo?.bar: Int get() = 1") {
		t.Fatal("extension receiver nullability must move the fingerprint")
	}
	if fp("val a.Foo.bar: Int get() = 1") == fp("val b.Foo.bar: Int get() = 1") {
		t.Fatal("extension receiver qualifier must move the fingerprint")
	}
	if fp("class R { val f: () -> Unit = null }") == fp("class R { val f: suspend () -> Unit = null }") {
		t.Fatal("suspend type modifier must move the fingerprint")
	}
	// Composable is referenced exactly once in both snippets, so only the
	// captured type modifier can move the fingerprint.
	if fp("class R { @Composable fun g() {}\nval f: () -> Unit = null }") == fp("class R { fun g() {}\nval f: @Composable () -> Unit = null }") {
		t.Fatal("type annotation must move the fingerprint")
	}
	// Inferred types are deliberately not tracked: the initializer is body-like.
	if fp("class R { val name = 0 }") != fp("class R { val name = \"\" }") {
		t.Fatal("initializer-only edit without a declared type must stay stable")
	}
}

func TestKotlinPropertyDeclaredTypes(t *testing.T) {
	cases := []struct{ src, recv, typ string }{
		{"val x: Int = 0", "", "Int"},
		{"val x: Map<String,  Int?>? = null", "", "Map<String, Int?>?"},
		{"val Foo<T>.bar: Int get() = 1", "Foo<T>", "Int"},
		{"val f: suspend (Int) -> Unit = null", "", "suspend (Int) -> Unit"},
		{"val f: @Composable () -> Unit = null", "", "@Composable () -> Unit"},
		{"val @Ann Foo.x: Int get() = 1", "@Ann Foo", "Int"},
		{"val <T> T.v: T & Any get() = 1", "T", "T & Any"},
		{"val v: T & Any = null", "", "T & Any"},
		{"val x = 0", "", ""},
		{"val x by lazy { 1 }", "", ""},
	}
	for _, c := range cases {
		f := parseKotlinSnippet(t, c.src)
		var recv, typ string
		f.FlatWalkAllNodes(0, func(idx uint32) {
			if f.FlatType(idx) == "property_declaration" {
				recv, typ = KotlinPropertyDeclaredTypes(f, idx)
			}
		})
		if recv != c.recv || typ != c.typ {
			t.Errorf("%q: got (%q, %q), want (%q, %q)", c.src, recv, typ, c.recv, c.typ)
		}
	}
}

func TestSweepStalePackDirs(t *testing.T) {
	cacheDir := t.TempDir()
	mk := func(name string) string {
		p := filepath.Join(cacheDir, name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	old1, old2 := mk("packs-v1"), mk("packs-v5")
	cur := mk(packSubdir)
	other := mk("packs-foo")
	newer := mk("packs-v99")
	(&packStore{cacheDir: cacheDir}).sweepLegacyShardsDir()
	for _, p := range []string{old1, old2} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", p)
		}
	}
	for _, p := range []string{cur, other, newer} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s should be kept: %v", p, err)
		}
	}
}
