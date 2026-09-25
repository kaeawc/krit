package module

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseVersionCatalog(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "libs.versions.toml")
	contents := `# top comment
[versions]
kotlin = "1.9.0"
okhttp = "4.12.0" # trailing comment

[libraries]
okhttp = { module = "com.squareup.okhttp3:okhttp", version.ref = "okhttp" }
kotlin-stdlib = { module = "org.jetbrains.kotlin:kotlin-stdlib", version.ref = "kotlin" }
unused-lib = "com.example:unused:1.0"

[plugins]
android-application = { id = "com.android.application", version = "8.1.0" }

[bundles]
network = ["okhttp", "kotlin-stdlib"]
`
	if err := os.WriteFile(catalogPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cat, err := ParseVersionCatalog(catalogPath)
	if err != nil {
		t.Fatalf("ParseVersionCatalog: %v", err)
	}
	if got, want := len(cat.Versions), 2; got != want {
		t.Errorf("versions: got %d, want %d", got, want)
	}
	if got, want := len(cat.Libraries), 3; got != want {
		t.Errorf("libraries: got %d, want %d", got, want)
	}
	if got, want := len(cat.Plugins), 1; got != want {
		t.Errorf("plugins: got %d, want %d", got, want)
	}
	if got, want := len(cat.Bundles), 1; got != want {
		t.Errorf("bundles: got %d, want %d", got, want)
	}
	wantLineForKotlinStdlib := 8
	for _, e := range cat.Libraries {
		if e.Alias == "kotlin-stdlib" && e.Line != wantLineForKotlinStdlib {
			t.Errorf("kotlin-stdlib line: got %d, want %d", e.Line, wantLineForKotlinStdlib)
		}
	}

	wantValues := map[string]string{"kotlin": "1.9.0", "okhttp": "4.12.0"}
	for _, e := range cat.Versions {
		if want, ok := wantValues[e.Alias]; ok && e.Value != want {
			t.Errorf("version %s value: got %q, want %q", e.Alias, e.Value, want)
		}
	}
	for _, e := range cat.Libraries {
		if e.Value != "" {
			t.Errorf("library %s value: got %q, want empty", e.Alias, e.Value)
		}
	}
}

func TestAccessorFor(t *testing.T) {
	cases := map[[2]string]string{
		{"libs", "okhttp"}:          "libs.okhttp",
		{"libs", "kotlin-stdlib"}:   "libs.kotlin.stdlib",
		{"libs", "androidx_core"}:   "libs.androidx.core",
		{"libs.plugins", "android"}: "libs.plugins.android",
	}
	for input, want := range cases {
		got := AccessorFor(input[0], input[1])
		if got != want {
			t.Errorf("AccessorFor(%q,%q) = %q, want %q", input[0], input[1], got, want)
		}
	}
}

func TestFindVersionCatalog(t *testing.T) {
	dir := t.TempDir()
	if got := FindVersionCatalog(dir); got != "" {
		t.Errorf("absent catalog: got %q, want empty", got)
	}
	gradleDir := filepath.Join(dir, "gradle")
	if err := os.MkdirAll(gradleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(gradleDir, "libs.versions.toml")
	if err := os.WriteFile(path, []byte("[versions]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := FindVersionCatalog(dir); got != path {
		t.Errorf("present catalog: got %q, want %q", got, path)
	}
}

func TestParseVersionCatalogSpecCompliantShapes(t *testing.T) {
	cases := []struct {
		name, content string
		wantLine      int
		wantVersion   string
	}{
		{"sub-table", "[versions]\nk = \"1.0\"\n[libraries.core]\nmodule = \"a:b\"\nversion.ref = \"k\"\n", 3, "1.0"},
		{"nested-inline-ref", "[versions]\nk = \"1.0\"\n[libraries]\ncore = { module = \"a:b\", version = { ref = \"k\" } }\n", 4, "1.0"},
		{"rich-version-prefer", "[versions]\nk = { strictly = \"[1.0,2.0)\", prefer = \"1.5\" }\n[libraries]\ncore = { module = \"a:b\", version.ref = \"k\" }\n", 4, "1.5"},
		{"literal-strings", "[versions]\nk = '1.0'\n[libraries]\ncore = { module = 'a:b', version.ref = 'k' }\n", 4, "1.0"},
		{"dotted-keys", "versions.k = \"1.0\"\nlibraries.core = { module = \"a:b\", version.ref = \"k\" }\n", 2, "1.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "libs.versions.toml")
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			cat, err := ParseVersionCatalog(path)
			if err != nil {
				t.Fatal(err)
			}
			var lib *CatalogEntry
			for i := range cat.Libraries {
				if cat.Libraries[i].Alias == "core" {
					lib = &cat.Libraries[i]
				}
			}
			if lib == nil || lib.Module != "a:b" || lib.Version != tc.wantVersion || lib.Line != tc.wantLine {
				t.Fatalf("core = %#v, want module a:b version %q line %d", lib, tc.wantVersion, tc.wantLine)
			}
		})
	}
}

func TestParseVersionCatalogRichVersionDuplicateCheckValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "libs.versions.toml")
	content := "[versions]\na = \"1.0\"\nb = \"1.0\"\nc = { strictly = \"1.0\" }\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := ParseVersionCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a": "1.0", "b": "1.0", "c": ""}
	for _, e := range cat.Versions {
		if got := want[e.Alias]; e.Value != got {
			t.Errorf("%s Value = %q, want %q", e.Alias, e.Value, got)
		}
	}
}

func TestParseVersionCatalogInvalidTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "libs.versions.toml")
	if err := os.WriteFile(path, []byte("[versions]\nk = \"1.0\"\nk = \"2.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseVersionCatalog(path); err == nil {
		t.Fatal("expected duplicate key parse error")
	}
}
