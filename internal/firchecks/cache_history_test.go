package firchecks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

func TestCacheEntriesPruneOldestFingerprintsPerPath(t *testing.T) {
	cacheDir, err := CacheDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entries := []*FirCacheEntry{
		{V: FirCacheVersion, FilePath: "/src/Stable.kt", ContentHash: "one", ClosureFingerprint: "first"},
		{V: FirCacheVersion, FilePath: "/src/Stable.kt", ContentHash: "two", ClosureFingerprint: "second"},
		{V: FirCacheVersion, FilePath: "/src/Stable.kt", ContentHash: "three", ClosureFingerprint: "third"},
		{V: FirCacheVersion, FilePath: "/src/Other.kt", ContentHash: "one", ClosureFingerprint: "first"},
	}
	for index, entry := range entries {
		if err := WriteCacheEntry(cacheDir, entry); err != nil {
			t.Fatal(err)
		}
		modified := time.Unix(int64(index+1), 0)
		if err := os.Chtimes(entryPath(cacheDir, entry.FilePath, entry.ContentHash, entry.ClosureFingerprint), modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteCacheEntry(cacheDir, entries[0]); err != nil {
		t.Fatal(err)
	}
	latest := &FirCacheEntry{V: FirCacheVersion, FilePath: "/src/Stable.kt", ContentHash: "four", ClosureFingerprint: "fourth"}
	if err := WriteCacheEntry(cacheDir, latest); err != nil {
		t.Fatal(err)
	}
	for index, entry := range append(entries, latest) {
		loaded, err := LoadCacheEntry(cacheDir, entry.FilePath, entry.ContentHash, entry.ClosureFingerprint)
		if err != nil {
			t.Fatal(err)
		}
		if (loaded == nil) != (index == 1) {
			t.Fatalf("fingerprint %s for %s: entry = %+v, want evicted = %v", entry.ClosureFingerprint, entry.FilePath, loaded, index == 1)
		}
	}
}

func TestInvokeCachedSourceEditDuringRunSkipsCacheWrite(t *testing.T) {
	for _, editedFile := range []string{"requested", "dependency"} {
		t.Run(editedFile, func(t *testing.T) {
			root := t.TempDir()
			requested := filepath.Join(root, "Requested.kt")
			dependency := filepath.Join(root, "Dependency.kt")
			original := []byte("fun value() = 1\n")
			for _, path := range []string{requested, dependency} {
				if err := os.WriteFile(path, original, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			changed := requested
			if editedFile == "dependency" {
				changed = dependency
			}
			calls := 0
			run := func(_ string, _, _, _, _ []string, _ RuleConfigs, _ FileFacts, _, _ bool, _ ...CompileTarget) (*CheckResponse, error) {
				calls++
				if calls == 1 {
					if err := os.WriteFile(changed, []byte("fun value() = 2\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return &CheckResponse{}, nil
			}
			invoke := func() {
				t.Helper()
				if _, err := invokeCachedWith(run, "", []string{requested}, []string{root}, nil, nil, nil, FileFacts{}, root, false, false); err != nil {
					t.Fatal(err)
				}
			}
			invoke()
			cacheDir, err := CacheDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(cacheDir, "entries")); !os.IsNotExist(err) {
				t.Fatalf("changed compilation wrote cache entries: %v", err)
			}
			if err := os.WriteFile(changed, original, 0o600); err != nil {
				t.Fatal(err)
			}
			invoke()
			invoke()
			if calls != 2 {
				t.Fatalf("checker calls = %d, want fresh run after revert then cache hit", calls)
			}
		})
	}
}

func TestCacheEntriesRetainFingerprintVariants(t *testing.T) {
	cacheDir, err := CacheDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entries := []*FirCacheEntry{
		{
			V:                  FirCacheVersion,
			ContentHash:        "same-content",
			FilePath:           "/src/Stable.kt",
			ClosureFingerprint: "original-inputs",
			Rules:              []string{"R"},
			Findings:           []FirFinding{{Path: "/src/Stable.kt", Rule: "R", Message: "original"}},
		},
		{
			V:                  FirCacheVersion,
			ContentHash:        "same-content",
			FilePath:           "/src/Stable.kt",
			ClosureFingerprint: "edited-inputs",
			Rules:              []string{"R"},
			ErrorMessage:       "compiler error after dependency edit",
			RuleErrors:         map[string]string{"R": "checker threw"},
		},
	}
	for _, entry := range entries {
		if err := WriteCacheEntry(cacheDir, entry); err != nil {
			t.Fatal(err)
		}
	}
	for _, entry := range entries {
		loaded, err := LoadCacheEntry(cacheDir, entry.FilePath, entry.ContentHash, entry.ClosureFingerprint)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(loaded, entry) {
			t.Fatalf("loaded = %+v, want %+v", loaded, entry)
		}
	}
	if entry, err := LoadCacheEntry(cacheDir, entries[0].FilePath, entries[0].ContentHash, "unknown-inputs"); err != nil || entry != nil {
		t.Fatalf("unknown fingerprint: entry = %+v, err = %v, want a miss", entry, err)
	}
}

func TestInvokeCachedEditRevertRetainsCompilationVerdicts(t *testing.T) {
	root := t.TempDir()
	changed := filepath.Join(root, "Changed.kt")
	stable := filepath.Join(root, "Stable.kt")
	files := []string{changed, stable}
	original := "fun value() = 1\n"
	edited := "fun value() = 2\n"
	for _, file := range files {
		if err := os.WriteFile(file, []byte(original), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	run := func(_ string, misses, _, _, _ []string, _ RuleConfigs, _ FileFacts, _, _ bool, _ ...CompileTarget) (*CheckResponse, error) {
		calls++
		if !reflect.DeepEqual(misses, files) {
			t.Fatalf("misses = %v, want conservative whole-compilation misses %v", misses, files)
		}
		content, err := os.ReadFile(changed)
		if err != nil {
			t.Fatal(err)
		}
		response := &CheckResponse{
			Rules:      []string{"R"},
			RuleErrors: map[string]map[string]string{"OtherRule": {stable: string(content)}},
		}
		for _, file := range misses {
			response.Findings = append(response.Findings, FirFinding{Path: file, Line: 1, Col: 1, Rule: "R", Message: string(content)})
		}
		return response, nil
	}
	for _, step := range []struct {
		name    string
		content string
		calls   int
	}{
		{"cold", original, 1},
		{"warm", original, 1},
		{"edit", edited, 2},
		{"edited-warm", edited, 2},
		{"revert", original, 2},
		{"reedit", edited, 2},
	} {
		t.Run(step.name, func(t *testing.T) {
			if err := os.WriteFile(changed, []byte(step.content), 0o600); err != nil {
				t.Fatal(err)
			}
			result, err := invokeCachedWith(run, "", files, []string{root}, nil, []string{"R"}, nil, FileFacts{}, root, false, false)
			if err != nil {
				t.Fatal(err)
			}
			if calls != step.calls {
				t.Fatalf("checker calls = %d, want %d", calls, step.calls)
			}
			if len(result.Findings) != len(files) {
				t.Fatalf("findings = %v, want one per file", result.Findings)
			}
			for index, finding := range result.Findings {
				if finding.File != files[index] || finding.Message != step.content {
					t.Fatalf("finding = %+v, want %s with message %q", finding, files[index], step.content)
				}
			}
			if !reflect.DeepEqual(result.Rules, []string{"R"}) || !reflect.DeepEqual(result.RuleErrors, map[string]map[string]string{"OtherRule": {stable: step.content}}) {
				t.Fatalf("cached verdict metadata changed: rules = %v, rule errors = %v", result.Rules, result.RuleErrors)
			}
		})
	}
}

func TestInvokeCachedCompileInputChangesMissAllFiles(t *testing.T) {
	for _, input := range []string{"classpath-content", "classpath-list", "jar", "rules", "rule-config", "jvm-target", "kotlin-settings"} {
		t.Run(input, func(t *testing.T) {
			root := t.TempDir()
			files := []string{filepath.Join(root, "A.kt"), filepath.Join(root, "B.kt")}
			jar := filepath.Join(root, "krit-fir.jar")
			dependency := filepath.Join(root, "dependency.jar")
			for _, file := range append(append([]string(nil), files...), jar, dependency) {
				if err := os.WriteFile(file, []byte("initial"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			classpath := []string{dependency}
			rules := []string{"R"}
			configs := RuleConfigs{"R": {"limit": 1}}
			target := CompileTarget{JvmTarget: "11", Kotlin: gradlemodel.KotlinSettings{LanguageVersion: "2.3"}}
			calls := 0
			run := func(_ string, misses, _, _, _ []string, _ RuleConfigs, _ FileFacts, _, _ bool, _ ...CompileTarget) (*CheckResponse, error) {
				calls++
				if !reflect.DeepEqual(misses, files) {
					t.Fatalf("misses = %v, want %v", misses, files)
				}
				return &CheckResponse{Rules: rules}, nil
			}
			check := func(wantCalls int) {
				t.Helper()
				if _, err := invokeCachedWith(run, jar, files, []string{root}, classpath, rules, configs, FileFacts{}, root, false, false, target); err != nil {
					t.Fatal(err)
				}
				if calls != wantCalls {
					t.Fatalf("checker calls = %d, want %d", calls, wantCalls)
				}
			}
			check(1)
			check(1)
			switch input {
			case "classpath-content", "jar":
				file := dependency
				if input == "jar" {
					file = jar
				}
				if err := os.WriteFile(file, []byte("changed compile input"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "classpath-list":
				classpath = nil
			case "rules":
				rules = []string{"OtherRule"}
			case "rule-config":
				configs = RuleConfigs{"R": {"limit": 2}}
			case "jvm-target":
				target.JvmTarget = "17"
			case "kotlin-settings":
				target.Kotlin.LanguageVersion = "2.4"
			}
			check(2)
			check(2)
		})
	}
}
