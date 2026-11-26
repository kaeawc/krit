package librarymodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeGradleTree(t *testing.T, root, kotlinVersion string) []string {
	t.Helper()
	files := map[string]string{
		"settings.gradle.kts":       "include(\":app\")\n",
		"gradle/libs.versions.toml": "[versions]\nkotlin = \"" + kotlinVersion + "\"\n[libraries]\nkotlin-stdlib = { module = \"org.jetbrains.kotlin:kotlin-stdlib\", version.ref = \"kotlin\" }\n",
		"app/build.gradle.kts":      "plugins { kotlin(\"jvm\") }\ndependencies { implementation(libs.kotlin.stdlib) }\nsourceSets { main { java.srcDirs(\"src/main/kotlin\") } }\n",
	}
	var paths []string
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return paths
}

// #1662: the same build at another absolute path has the same library facts
// fingerprint, so the analysis cache keyed by it survives a CI restore into
// another workspace; a real change still moves it.
func TestFactsFingerprintIsRelocatable(t *testing.T) {
	base := t.TempDir()
	a := FactsForProfile(ProfileFromPaths(writeGradleTree(t, filepath.Join(base, "agent-1", "repo"), "2.3.0"))).Fingerprint()
	b := FactsForProfile(ProfileFromPaths(writeGradleTree(t, filepath.Join(base, "agent-2", "repo"), "2.3.0"))).Fingerprint()
	if a != b {
		t.Fatal("the same build at another path has a different facts fingerprint")
	}
	c := FactsForProfile(ProfileFromPaths(writeGradleTree(t, filepath.Join(base, "agent-3", "repo"), "2.4.0"))).Fingerprint()
	if c == a {
		t.Fatal("a version change kept the facts fingerprint")
	}
}

// Facts loaded back from the library-model cache fingerprint the same as
// freshly built ones, or every warm run after a cold one misses the
// analysis cache.
func TestFactsFingerprintSurvivesJSONRoundTrip(t *testing.T) {
	f := FactsForProfile(ProfileFromPaths(writeGradleTree(t, filepath.Join(t.TempDir(), "repo"), "2.3.0")))
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	var back Facts
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Fingerprint() != f.Fingerprint() {
		t.Fatal("a JSON round trip changed the facts fingerprint")
	}
}
