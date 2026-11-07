package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestGradleSourceMasksCommentsAndStrings(t *testing.T) {
	content := "a // kotlinOptions\n/* kotlinOptions\n x */ b \"kotlinOptions\" 'c' \"\"\"\nkotlinOptions\n\"\"\" d ${x}\n\"a${\"q\"}b\" e\n"
	src := newGradleSource(content)
	if len(src.code) != len(content) || len(src.text) != len(content) {
		t.Fatalf("masked views must keep the original length")
	}
	if strings.Contains(src.code, "kotlinOptions") {
		t.Fatalf("code view still holds kotlinOptions: %q", src.code)
	}
	if !strings.Contains(src.text, `"kotlinOptions"`) {
		t.Fatalf("text view must keep string literals: %q", src.text)
	}
	if strings.Contains(src.text, "// kotlinOptions") || strings.Contains(src.text, "/* kotlinOptions") {
		t.Fatalf("text view must blank comments: %q", src.text)
	}
	if !strings.Contains(src.code, " e") {
		t.Fatalf("code after a nested template string was lost: %q", src.code)
	}
	if line, col := src.lineCol(strings.Index(content, "b \"")); line != 3 || col != 7 {
		t.Fatalf("lineCol = %d:%d, want 3:7", line, col)
	}
}

func TestParseGradleVersion(t *testing.T) {
	cases := []struct {
		in           string
		maj, min, pt int
		known        bool
	}{
		{"2.4.20", 2, 4, 20, true},
		{"2.2", 2, 2, 0, true},
		{"9.0.0-alpha05", 9, 0, 0, true},
		{"2.0.20-Beta1", 2, 0, 20, true},
		{"2.3.0-RC2", 2, 3, 0, true},
		{"", 0, 0, 0, false},
		{"latest", 0, 0, 0, false},
	}
	for _, c := range cases {
		v := parseGradleVersion(c.in)
		if v.known != c.known || v.major != c.maj || v.minor != c.min || v.patch != c.pt {
			t.Errorf("parseGradleVersion(%q) = %+v", c.in, v)
		}
	}
	if !parseGradleVersion("2.3.20").atLeast(2, 3, 20) || parseGradleVersion("2.3.10").atLeast(2, 3, 20) || parseGradleVersion("").atLeast(0, 0, 0) {
		t.Fatal("atLeast ordering is wrong")
	}
}

func TestGradleToolchainFromCatalog(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "gradle"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalog := "[versions]\nkotlin = \"2.4.20\"\nagp = \"9.1.0\"\n[plugins]\nkotlin-android = { id = \"org.jetbrains.kotlin.android\", version.ref = \"kotlin\" }\nandroid-app = { id = \"com.android.application\", version.ref = \"agp\" }\n"
	if err := os.WriteFile(filepath.Join(dir, "gradle", "libs.versions.toml"), []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "plugins {\n    alias(libs.plugins.android.app)\n    alias(libs.plugins.kotlin.android)\n}\n"
	path := filepath.Join(dir, "build.gradle.kts")
	tool := gradleToolchainFor(&api.Context{GradlePath: path, GradleContent: script})
	if !tool.kotlin.atLeast(2, 4, 20) || !tool.agp.atLeast(9, 1, 0) || tool.agp.atLeast(9, 2, 0) {
		t.Fatalf("toolchain = %+v", tool)
	}
	if got := gradleCatalogPluginID(&api.Context{GradlePath: path}, "kotlin.android"); got != "org.jetbrains.kotlin.android" {
		t.Fatalf("catalog plugin id = %q", got)
	}
}

func TestGradlePropertiesChainStopsAtSettings(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "app")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, c string) {
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "settings.gradle.kts"), "")
	write(filepath.Join(root, "gradle.properties"), "# krit:ignore Foo\nandroid.builtInKotlin=false\n")
	write(filepath.Join(mod, "build.gradle.kts"), "")
	files := GradlePropertiesFilesFor(filepath.Join(mod, "build.gradle.kts"))
	if len(files) != 1 || files[0] != filepath.Join(root, "gradle.properties") {
		t.Fatalf("GradlePropertiesFilesFor = %v", files)
	}
	if v, ok := gradlePropertyValue(filepath.Join(mod, "build.gradle.kts"), "android.builtInKotlin"); !ok || v != "false" {
		t.Fatalf("property = %q, %v", v, ok)
	}
	props := readGradleProps(files[0])
	if len(props.entries) != 1 || !gradlePropSuppressed(props.entries[0], "Foo") || gradlePropSuppressed(props.entries[0], "Bar") {
		t.Fatalf("entries = %+v", props.entries)
	}
}
