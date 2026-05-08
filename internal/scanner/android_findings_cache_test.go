package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleAndroidFindings() FindingColumns {
	c := NewFindingCollector(2)
	c.Append(Finding{
		File:     "AndroidManifest.xml",
		Line:     12,
		Col:      4,
		RuleSet:  "android",
		Rule:     "ExportedActivity",
		Severity: "warning",
		Message:  "Activity is exported without a permission",
	})
	c.Append(Finding{
		File:     "build.gradle.kts",
		Line:     34,
		Col:      8,
		RuleSet:  "gradle",
		Rule:     "MissingNamespace",
		Severity: "error",
		Message:  "android.namespace is required",
	})
	return *c.Columns()
}

func TestAndroidFindingsKey_StableForEqualInputs(t *testing.T) {
	in := AndroidFindingsKeyInputs{
		Kind:                AndroidFindingsKindManifest,
		RuleHash:            "rh-aaa",
		LibraryFactsFP:      "lf-bbb",
		JavaSemanticFactsFP: "jf-ccc",
		InputFP:             "if-ddd",
		Extra:               "ex-eee",
	}
	// Two independent calls with identical input must produce identical
	// keys; this catches accidental non-determinism (e.g., a hash that
	// folds in time.Now or rand). The seemingly-redundant double call
	// is the intended check.
	a := AndroidFindingsKey(in)
	b := AndroidFindingsKey(in)
	if a != b {
		t.Fatal("equal inputs produced different keys")
	}
}

func TestAndroidFindingsKey_KindIsolation(t *testing.T) {
	base := AndroidFindingsKeyInputs{
		Kind:                AndroidFindingsKindManifest,
		RuleHash:            "rh",
		LibraryFactsFP:      "lf",
		JavaSemanticFactsFP: "jf",
		InputFP:             "if",
	}
	other := base
	other.Kind = AndroidFindingsKindGradle
	if AndroidFindingsKey(base) == AndroidFindingsKey(other) {
		t.Fatal("manifest and gradle keys collided for identical InputFP — kind tag not separating families")
	}
}

func TestAndroidFindingsKey_SensitiveToEachField(t *testing.T) {
	base := AndroidFindingsKeyInputs{
		Kind:                AndroidFindingsKindManifest,
		RuleHash:            "rh",
		LibraryFactsFP:      "lf",
		JavaSemanticFactsFP: "jf",
		InputFP:             "if",
		Extra:               "ex",
	}
	mut := []func(*AndroidFindingsKeyInputs){
		func(i *AndroidFindingsKeyInputs) { i.RuleHash = "rh2" },
		func(i *AndroidFindingsKeyInputs) { i.LibraryFactsFP = "lf2" },
		func(i *AndroidFindingsKeyInputs) { i.JavaSemanticFactsFP = "jf2" },
		func(i *AndroidFindingsKeyInputs) { i.InputFP = "if2" },
		func(i *AndroidFindingsKeyInputs) { i.Extra = "ex2" },
	}
	baseKey := AndroidFindingsKey(base)
	for i, m := range mut {
		v := base
		m(&v)
		if AndroidFindingsKey(v) == baseKey {
			t.Fatalf("mutation %d should change the key", i)
		}
	}
}

func TestAndroidFindings_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	key := AndroidFindingsKey(AndroidFindingsKeyInputs{
		Kind:    AndroidFindingsKindManifest,
		InputFP: "abc",
	})
	cols := sampleAndroidFindings()
	if err := SaveAndroidFindings(dir, key, cols); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok := LoadAndroidFindings(dir, key)
	if !ok {
		t.Fatal("expected cache hit after save")
	}
	if got.N != cols.N {
		t.Fatalf("row count mismatch: got %d want %d", got.N, cols.N)
	}
	for i := range cols.Files {
		if got.Files[i] != cols.Files[i] {
			t.Fatalf("file %d mismatch: got %q want %q", i, got.Files[i], cols.Files[i])
		}
	}
	for i := range cols.Rules {
		if got.Rules[i] != cols.Rules[i] {
			t.Fatalf("rule %d mismatch: got %q want %q", i, got.Rules[i], cols.Rules[i])
		}
	}
}

func TestAndroidFindings_MissOnUnknownKey(t *testing.T) {
	dir := t.TempDir()
	if _, ok := LoadAndroidFindings(dir, "doesnotexist"); ok {
		t.Fatal("expected miss for absent key")
	}
}

func TestAndroidFindings_VersionMismatchIsMiss(t *testing.T) {
	dir := t.TempDir()
	key := "deadbeef0001"
	if err := SaveAndroidFindings(dir, key, sampleAndroidFindings()); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Corrupt the version field (bytes 4..7) on disk to simulate a
	// schema bump.
	path := androidEntryPath(dir, key)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	data[4] = 0xff
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, ok := LoadAndroidFindings(dir, key); ok {
		t.Fatal("expected miss after version corruption")
	}
}

func TestAndroidFindings_CRCFailureIsMiss(t *testing.T) {
	dir := t.TempDir()
	key := "deadbeef0002"
	if err := SaveAndroidFindings(dir, key, sampleAndroidFindings()); err != nil {
		t.Fatalf("save: %v", err)
	}
	path := androidEntryPath(dir, key)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Flip the last byte (in compressed payload) to break CRC and
	// decode.
	data[len(data)-1] ^= 0xff
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, ok := LoadAndroidFindings(dir, key); ok {
		t.Fatal("expected miss after CRC corruption")
	}
}

func TestAndroidFindings_ClearRemovesEntries(t *testing.T) {
	dir := t.TempDir()
	if err := SaveAndroidFindings(dir, "aabb1234", sampleAndroidFindings()); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := ClearAndroidFindingsCache(dir); err != nil {
		t.Fatalf("clear: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected empty dir after clear, found: %s", strings.Join(names, ","))
	}
}

func TestAndroidFindings_EntryPathSharded(t *testing.T) {
	dir := "/tmp/krit-test"
	got := androidEntryPath(dir, "deadbeef")
	want := filepath.Join(dir, "entries", "de", "adbeef.bin")
	if got != want {
		t.Fatalf("entry path: got %q want %q", got, want)
	}
}
