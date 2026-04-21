package android

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
)

const sampleValuesXML = `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="hello">Hello</string>
    <string name="world" translatable="false">World</string>
    <string name="fmt" formatted="false">Weird %{var}</string>
    <color name="primary">#FF0000</color>
    <dimen name="pad">16dp</dimen>
    <integer name="count">7</integer>
    <bool name="flag">true</bool>
    <string-array name="fruits">
        <item>apple</item>
        <item>banana</item>
    </string-array>
    <plurals name="items">
        <item quantity="one">%d item</item>
        <item quantity="other">%d items</item>
    </plurals>
    <style name="AppTheme" parent="Theme.Material">
        <item name="colorPrimary">@color/primary</item>
    </style>
</resources>
`

const sampleColorsXML = `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <color name="accent">#00FF00</color>
    <color name="background">#FFFFFF</color>
</resources>
`

func writeSampleValuesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	valuesDir := filepath.Join(dir, "values")
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		t.Fatalf("mkdir values: %v", err)
	}
	if err := os.WriteFile(filepath.Join(valuesDir, "strings.xml"), []byte(sampleValuesXML), 0o644); err != nil {
		t.Fatalf("write strings.xml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(valuesDir, "colors.xml"), []byte(sampleColorsXML), 0o644); err != nil {
		t.Fatalf("write colors.xml: %v", err)
	}
	return valuesDir
}

// withResourceCache installs a fresh cache rooted at repoDir and
// restores the previous active cache on cleanup.
func withResourceCache(t *testing.T, repoDir string) *ResourceIndexCache {
	t.Helper()
	prev := ActiveResourceIndexCache()
	c, err := NewResourceIndexCache(repoDir)
	if err != nil {
		t.Fatalf("NewResourceIndexCache: %v", err)
	}
	SetActiveResourceIndexCache(c)
	t.Cleanup(func() {
		_ = c.Close()
		SetActiveResourceIndexCache(prev)
	})
	return c
}

// sortStringSlice returns a copy sorted ascending so Drawables /
// ExtraText comparisons don't fail on scan order.
func sortStringSlice(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func TestResourceIndexCache_RoundTripFindingEquivalence(t *testing.T) {
	valuesDir := writeSampleValuesDir(t)

	// Cold run (no cache)
	prev := ActiveResourceIndexCache()
	SetActiveResourceIndexCache(nil)
	cold, _, err := scanValuesDirIndexKinds(valuesDir, runtime.NumCPU(), ValuesScanAll)
	if err != nil {
		t.Fatalf("cold scan: %v", err)
	}
	SetActiveResourceIndexCache(prev)

	// Prime cache
	repo := t.TempDir()
	cache := withResourceCache(t, repo)

	warm1, _, err := scanValuesDirIndexKinds(valuesDir, runtime.NumCPU(), ValuesScanAll)
	if err != nil {
		t.Fatalf("prime scan: %v", err)
	}
	if err := cache.Close(); err != nil {
		t.Fatalf("Close after prime scan: %v", err)
	}
	if s := cache.Stats(); s.Hits != 0 || s.Misses != 1 {
		t.Errorf("after prime: hits=%d misses=%d (want 0/1)", s.Hits, s.Misses)
	}

	// Warm hit
	warm2, _, err := scanValuesDirIndexKinds(valuesDir, runtime.NumCPU(), ValuesScanAll)
	if err != nil {
		t.Fatalf("warm scan: %v", err)
	}
	if s := cache.Stats(); s.Hits != 1 || s.Misses != 1 {
		t.Errorf("after warm hit: hits=%d misses=%d (want 1/1)", s.Hits, s.Misses)
	}
	assertAnyResourceCacheEntryZstd(t, cache)

	// Normalize slice order for comparison (maps are map-order already).
	normalize := func(r *ResourceIndex) {
		r.Drawables = sortStringSlice(r.Drawables)
	}
	normalize(cold)
	normalize(warm1)
	normalize(warm2)

	if !reflect.DeepEqual(cold.Strings, warm2.Strings) {
		t.Errorf("Strings drift cold vs warm:\n cold=%#v\n warm=%#v", cold.Strings, warm2.Strings)
	}
	if !reflect.DeepEqual(cold.StringsLocation, warm2.StringsLocation) {
		t.Errorf("StringsLocation drift (position info must round-trip):\n cold=%#v\n warm=%#v",
			cold.StringsLocation, warm2.StringsLocation)
	}
	if !reflect.DeepEqual(cold.Colors, warm2.Colors) {
		t.Errorf("Colors drift: cold=%v warm=%v", cold.Colors, warm2.Colors)
	}
	if !reflect.DeepEqual(cold.Dimensions, warm2.Dimensions) {
		t.Errorf("Dimensions drift: cold=%v warm=%v", cold.Dimensions, warm2.Dimensions)
	}
	if !reflect.DeepEqual(cold.Integers, warm2.Integers) {
		t.Errorf("Integers drift: cold=%v warm=%v", cold.Integers, warm2.Integers)
	}
	if !reflect.DeepEqual(cold.Booleans, warm2.Booleans) {
		t.Errorf("Booleans drift: cold=%v warm=%v", cold.Booleans, warm2.Booleans)
	}
	if !reflect.DeepEqual(cold.StringArrays, warm2.StringArrays) {
		t.Errorf("StringArrays drift: cold=%v warm=%v", cold.StringArrays, warm2.StringArrays)
	}
	if !reflect.DeepEqual(cold.Plurals, warm2.Plurals) {
		t.Errorf("Plurals drift: cold=%v warm=%v", cold.Plurals, warm2.Plurals)
	}
	if !reflect.DeepEqual(cold.StringsNonTranslate, warm2.StringsNonTranslate) {
		t.Errorf("StringsNonTranslate drift: cold=%v warm=%v",
			cold.StringsNonTranslate, warm2.StringsNonTranslate)
	}
	if !reflect.DeepEqual(cold.StringsNonFormatted, warm2.StringsNonFormatted) {
		t.Errorf("StringsNonFormatted drift: cold=%v warm=%v",
			cold.StringsNonFormatted, warm2.StringsNonFormatted)
	}
	if len(cold.Styles) != len(warm2.Styles) {
		t.Errorf("Styles count drift: cold=%d warm=%d", len(cold.Styles), len(warm2.Styles))
	}
	for name, s := range cold.Styles {
		w := warm2.Styles[name]
		if w == nil {
			t.Errorf("Styles[%q] missing on warm", name)
			continue
		}
		if s.Name != w.Name || s.Parent != w.Parent || !reflect.DeepEqual(s.Items, w.Items) {
			t.Errorf("Styles[%q] drift: cold=%#v warm=%#v", name, s, w)
		}
	}
}

func assertAnyResourceCacheEntryZstd(t *testing.T, cache *ResourceIndexCache) {
	t.Helper()
	var checked bool
	err := filepath.WalkDir(cache.entriesDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || filepath.Ext(path) != resourceCacheExt || checked {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read resource cache entry: %v", err)
		}
		if !cacheutil.IsZstdFrame(data) {
			t.Fatalf("resource cache entry is not zstd-framed: %x", data[:min(4, len(data))])
		}
		checked = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk resource cache entries: %v", err)
	}
	if !checked {
		t.Fatal("expected at least one resource cache entry")
	}
}

func TestResourceIndexCache_InvalidatesOnFileChange(t *testing.T) {
	valuesDir := writeSampleValuesDir(t)
	repo := t.TempDir()
	cache := withResourceCache(t, repo)

	if _, _, err := scanValuesDirIndexKinds(valuesDir, 2, ValuesScanAll); err != nil {
		t.Fatalf("prime: %v", err)
	}
	if err := cache.Close(); err != nil {
		t.Fatalf("Close after prime: %v", err)
	}
	if _, _, err := scanValuesDirIndexKinds(valuesDir, 2, ValuesScanAll); err != nil {
		t.Fatalf("warm1: %v", err)
	}
	if s := cache.Stats(); s.Hits != 1 {
		t.Fatalf("warm1 hits: got %d want 1", s.Hits)
	}

	// Mutate one file — fingerprint changes → must miss.
	mutated := `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="hello">CHANGED</string>
</resources>
`
	if err := os.WriteFile(filepath.Join(valuesDir, "strings.xml"), []byte(mutated), 0o644); err != nil {
		t.Fatalf("mutate: %v", err)
	}
	idx, _, err := scanValuesDirIndexKinds(valuesDir, 2, ValuesScanAll)
	if err != nil {
		t.Fatalf("after-mutate: %v", err)
	}
	if got := idx.Strings["hello"]; got != "CHANGED" {
		t.Errorf("Strings[hello] after mutation = %q want CHANGED (stale cache served)", got)
	}
	if s := cache.Stats(); s.Misses != 2 {
		t.Errorf("after mutation expected misses=2 (prime + post-mutate), got %d", s.Misses)
	}
}

func TestResourceIndexCache_InvalidatesOnNewFile(t *testing.T) {
	valuesDir := writeSampleValuesDir(t)
	repo := t.TempDir()
	cache := withResourceCache(t, repo)

	if _, _, err := scanValuesDirIndexKinds(valuesDir, 2, ValuesScanAll); err != nil {
		t.Fatalf("prime: %v", err)
	}

	// Adding a new values XML must shift the fingerprint.
	extra := `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <color name="late_add">#123456</color>
</resources>
`
	if err := os.WriteFile(filepath.Join(valuesDir, "late.xml"), []byte(extra), 0o644); err != nil {
		t.Fatalf("add file: %v", err)
	}
	idx, _, err := scanValuesDirIndexKinds(valuesDir, 2, ValuesScanAll)
	if err != nil {
		t.Fatalf("post-add: %v", err)
	}
	if got := idx.Colors["late_add"]; got != "#123456" {
		t.Errorf("new file Colors[late_add] = %q want #123456 (stale cache?)", got)
	}
	if s := cache.Stats(); s.Misses != 2 {
		t.Errorf("adding a file must miss; got misses=%d", s.Misses)
	}
}

func TestResourceIndexCache_KindMaskInKey(t *testing.T) {
	valuesDir := writeSampleValuesDir(t)
	repo := t.TempDir()
	cache := withResourceCache(t, repo)

	if _, _, err := scanValuesDirIndexKinds(valuesDir, 2, ValuesScanAll); err != nil {
		t.Fatalf("scan all: %v", err)
	}
	// Different kinds mask → should miss, not reuse the all-kinds entry.
	if _, _, err := scanValuesDirIndexKinds(valuesDir, 2, ValuesScanStrings); err != nil {
		t.Fatalf("scan strings: %v", err)
	}
	if s := cache.Stats(); s.Hits != 0 || s.Misses != 2 {
		t.Errorf("kind-separated keys expected hits=0 misses=2, got hits=%d misses=%d",
			s.Hits, s.Misses)
	}
}

func TestResourceIndexCache_SaveAsyncFlushesClonedIndex(t *testing.T) {
	valuesDir := writeSampleValuesDir(t)
	repo := t.TempDir()
	cache := withResourceCache(t, repo)

	files, err := readValuesDirFiles(valuesDir, 2)
	if err != nil {
		t.Fatalf("readValuesDirFiles: %v", err)
	}
	fingerprint := computeValuesDirFingerprint(valuesDir, ValuesScanAll, files)
	idx := newResourceIndex()
	idx.Strings["hello"] = "Hello"
	idx.Styles["Theme"] = &Style{Items: map[string]string{"colorPrimary": "@color/primary"}}

	if err := cache.SaveAsync(fingerprint, ValuesScanAll, valuesDir, idx); err != nil {
		t.Fatalf("SaveAsync: %v", err)
	}
	idx.Strings["hello"] = "MUTATED"
	idx.Styles["Theme"].Items["colorPrimary"] = "@color/mutated"
	if err := cache.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, ok := cache.Load(fingerprint, ValuesScanAll, valuesDir)
	if !ok {
		t.Fatal("expected flushed async cache hit")
	}
	if got.Strings["hello"] != "Hello" {
		t.Fatalf("cached string = %q, want original clone", got.Strings["hello"])
	}
	if got.Styles["Theme"].Items["colorPrimary"] != "@color/primary" {
		t.Fatalf("cached style item = %q, want original clone", got.Styles["Theme"].Items["colorPrimary"])
	}
}

func TestResourceIndexCache_ResDirInKey(t *testing.T) {
	repo := t.TempDir()
	cache := withResourceCache(t, repo)

	// Two temp dirs with identical content → different resDir → distinct keys.
	make := func() string {
		d := t.TempDir()
		values := filepath.Join(d, "values")
		_ = os.MkdirAll(values, 0o755)
		_ = os.WriteFile(filepath.Join(values, "strings.xml"), []byte(sampleValuesXML), 0o644)
		return values
	}
	a, b := make(), make()
	if _, _, err := scanValuesDirIndexKinds(a, 2, ValuesScanAll); err != nil {
		t.Fatal(err)
	}
	if _, _, err := scanValuesDirIndexKinds(b, 2, ValuesScanAll); err != nil {
		t.Fatal(err)
	}
	if s := cache.Stats(); s.Misses != 2 {
		t.Errorf("identical content in different dirs must key separately (StringsLocation paths differ); got misses=%d", s.Misses)
	}
}

func TestResourceIndexCache_ClearWipesEntries(t *testing.T) {
	valuesDir := writeSampleValuesDir(t)
	repo := t.TempDir()
	cache := withResourceCache(t, repo)

	if _, _, err := scanValuesDirIndexKinds(valuesDir, 2, ValuesScanAll); err != nil {
		t.Fatal(err)
	}
	if err := cache.Close(); err != nil {
		t.Fatalf("Close after prime: %v", err)
	}
	if s := cache.Stats(); s.Entries == 0 {
		t.Fatalf("expected non-zero entries after prime, got %d", s.Entries)
	}
	if err := cache.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if s := cache.Stats(); s.Entries != 0 {
		t.Errorf("Clear should zero entries, got %d", s.Entries)
	}
	// A post-Clear scan must miss.
	if _, _, err := scanValuesDirIndexKinds(valuesDir, 2, ValuesScanAll); err != nil {
		t.Fatal(err)
	}
	// 1 prime-miss + 1 post-clear miss = 2 total misses; hits stays 0.
	if s := cache.Stats(); s.Hits != 0 {
		t.Errorf("Clear did not invalidate on-disk entries; hits=%d", s.Hits)
	}
}

func TestClearResourceIndexCache_RegisteredClearsDir(t *testing.T) {
	valuesDir := writeSampleValuesDir(t)
	repo := t.TempDir()
	_ = withResourceCache(t, repo)
	if _, _, err := scanValuesDirIndexKinds(valuesDir, 2, ValuesScanAll); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(repo, ".krit", resourceCacheDirName)
	if _, err := os.Stat(cacheDir); err != nil {
		t.Fatalf("cache dir missing after prime: %v", err)
	}
	if err := ClearResourceIndexCache(repo); err != nil {
		t.Fatalf("ClearResourceIndexCache: %v", err)
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Errorf("cache dir not removed: %v", err)
	}
}

func TestComputeValuesDirFingerprint_StableAndSensitive(t *testing.T) {
	f := []valuesDirFile{
		{path: "/r/values/a.xml", content: []byte("a")},
		{path: "/r/values/b.xml", content: []byte("b")},
	}
	base := computeValuesDirFingerprint("/r/values", ValuesScanAll, f)
	if base == "" {
		t.Fatal("empty fingerprint")
	}
	// Same inputs → same fingerprint.
	if again := computeValuesDirFingerprint("/r/values", ValuesScanAll, f); again != base {
		t.Errorf("unstable: %q vs %q", base, again)
	}
	// Different resDir.
	if diff := computeValuesDirFingerprint("/r/values-night", ValuesScanAll, f); diff == base {
		t.Error("resDir change did not shift fingerprint")
	}
	// Different kinds.
	if diff := computeValuesDirFingerprint("/r/values", ValuesScanStrings, f); diff == base {
		t.Error("kinds change did not shift fingerprint")
	}
	// Different content.
	f2 := []valuesDirFile{
		{path: "/r/values/a.xml", content: []byte("a")},
		{path: "/r/values/b.xml", content: []byte("b2")},
	}
	if diff := computeValuesDirFingerprint("/r/values", ValuesScanAll, f2); diff == base {
		t.Error("content change did not shift fingerprint")
	}
	// Added file.
	f3 := append(append([]valuesDirFile(nil), f...), valuesDirFile{path: "/r/values/c.xml", content: []byte("c")})
	if diff := computeValuesDirFingerprint("/r/values", ValuesScanAll, f3); diff == base {
		t.Error("new file did not shift fingerprint")
	}
}
