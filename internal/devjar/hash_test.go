package devjar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceHashTracksInputsAndTool(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{
		"LICENSE", "tools/THIRD_PARTY_NOTICES.txt",
		"tools/krit-rule-api/build.gradle.kts", "tools/krit-rule-api/settings.gradle.kts",
		"tools/krit-rule-api/gradle.properties", "tools/krit-rule-api/gradle/wrapper/gradle-wrapper.properties",
		"tools/krit-rule-api/src/main/kotlin/Api.kt",
	} {
		write(rel, "original")
	}
	for _, tool := range []string{"krit-fir", "krit-types"} {
		for _, rel := range []string{"build.gradle.kts", "settings.gradle.kts", "gradle.properties", "gradle/wrapper/gradle-wrapper.properties", "src/main/kotlin/Main.kt"} {
			write(filepath.Join("tools", tool, rel), "original")
		}
	}
	first, err := SourceHash(root, "krit-fir")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 20 {
		t.Fatalf("hash length = %d, want 20", len(first))
	}
	again, err := SourceHash(root, "krit-fir")
	if err != nil || again != first {
		t.Fatalf("same inputs: hash = %q, err = %v; want %q", again, err, first)
	}
	types, err := SourceHash(root, "krit-types")
	if err != nil || types == first {
		t.Fatalf("different tool: hash = %q, err = %v", types, err)
	}
	write("tools/krit-fir/src/main/kotlin/Main.kt", "changed")
	changed, err := SourceHash(root, "krit-fir")
	if err != nil || changed == first {
		t.Fatalf("changed source: hash = %q, err = %v", changed, err)
	}
	unchangedTypes, err := SourceHash(root, "krit-types")
	if err != nil || unchangedTypes != types {
		t.Fatalf("unrelated tool: hash = %q, err = %v; want %q", unchangedTypes, err, types)
	}
	write("tools/krit-rule-api/gradle.properties", "changed")
	changedAgain, err := SourceHash(root, "krit-fir")
	if err != nil || changedAgain == changed {
		t.Fatalf("shared build input: hash = %q, err = %v", changedAgain, err)
	}
}

func TestSourceHashRejectsUnknownTool(t *testing.T) {
	if _, err := SourceHash(t.TempDir(), "unknown"); err == nil {
		t.Fatal("expected unsupported tool error")
	}
}

func minimalSourceTree(t *testing.T) (string, func(rel, content string)) {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{
		"LICENSE", "tools/THIRD_PARTY_NOTICES.txt",
		"tools/krit-rule-api/build.gradle.kts", "tools/krit-rule-api/settings.gradle.kts",
		"tools/krit-rule-api/gradle.properties", "tools/krit-rule-api/gradle/wrapper/gradle-wrapper.properties",
		"tools/krit-fir/build.gradle.kts", "tools/krit-fir/settings.gradle.kts",
		"tools/krit-fir/gradle.properties", "tools/krit-fir/gradle/wrapper/gradle-wrapper.properties",
		"tools/krit-rule-api/src/main/kotlin/Api.kt", "tools/krit-fir/src/main/kotlin/Main.kt",
	} {
		write(rel, "original")
	}
	return root, write
}

// Concatenating raw "path\0content\0" records let one file's bytes mimic a
// record boundary, so these two different trees used to hash the same.
func TestSourceHashRecordsAreUnambiguous(t *testing.T) {
	const a = "tools/krit-fir/src/main/resources/a.bin"
	const b = "tools/krit-fir/src/main/resources/b.bin"

	merged, writeMerged := minimalSourceTree(t)
	writeMerged(a, "A\x00"+b+"\x00B")
	split, writeSplit := minimalSourceTree(t)
	writeSplit(a, "A")
	writeSplit(b, "B")

	h1, err := SourceHash(merged, "krit-fir")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := SourceHash(split, "krit-fir")
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Fatalf("different source trees share hash %q", h1)
	}
}

func TestSourceHashTracksKritVersion(t *testing.T) {
	root, _ := minimalSourceTree(t)
	t.Setenv("KRIT_VERSION", "")
	unset, err := SourceHash(root, "krit-fir")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_VERSION", "1.2.3")
	pinned, err := SourceHash(root, "krit-fir")
	if err != nil {
		t.Fatal(err)
	}
	if pinned == unset {
		t.Fatal("KRIT_VERSION did not change the dev-jar cache key")
	}
}
