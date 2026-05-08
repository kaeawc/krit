package module

import (
	"os"
	"path/filepath"
	"strings"
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
		if e.Alias == "unused-lib" && e.Value != "com.example:unused:1.0" {
			t.Errorf("unused-lib value: got %q, want %q", e.Value, "com.example:unused:1.0")
		}
		if e.Alias == "okhttp" && !strings.HasPrefix(e.Value, "{") {
			t.Errorf("okhttp inline-table value should retain braces; got %q", e.Value)
		}
	}
}

func TestStripLineComment(t *testing.T) {
	cases := map[string]string{
		`foo = "bar" # comment`:          `foo = "bar" `,
		`foo = "value with # inside"`:    `foo = "value with # inside"`,
		`# whole line`:                   ``,
		`val = 'single # inside string'`: `val = 'single # inside string'`,
		`plain = "no comment"`:           `plain = "no comment"`,
	}
	for input, want := range cases {
		got := stripLineComment(input)
		if got != want {
			t.Errorf("stripLineComment(%q) = %q, want %q", input, got, want)
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
