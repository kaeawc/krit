package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeLintTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// element-x-android keeps tools/lint/lint.xml (referenced from build logic);
// iosched has mobile/lint.xml and a lint { disable += listOf(...) } block.
func TestDiscoverAndroidLintReadsLintXMLAndGradleBlocks(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, "tools/lint/lint.xml", `<?xml version="1.0" encoding="UTF-8"?>
<lint>
    <issue id="MissingTranslation" severity="ignore" />
    <issue id="TypographyEllipsis" severity="error" />
    <issue id="IconMissingDensityFolder,IconDensities" severity="IGNORE" />
    <issue id="all" severity="ignore" />
    <issue id="HardcodedText">
        <ignore path="src/debug" />
    </issue>
</lint>
`)
	writeLintTestFile(t, root, "shared/build.gradle.kts", `
android {
    lint {
        disable += listOf("InvalidPackage", "GradleDependency")
        disable += "Typos" // trailing comment "NotAnId"
        checkReleaseBuilds = false
    }
}
dependencies { implementation("a:b:1") }
`)
	writeLintTestFile(t, root, "legacy/build.gradle", `
android {
    lintOptions {
        disable 'ObsoleteLintCustomCheck', "UnusedResources"
        abortOnError false
    }
}
`)
	writeLintTestFile(t, root, "app/src/test/resources/lint.xml", `<lint><issue id="FromFixture" severity="ignore"/></lint>`)
	writeLintTestFile(t, root, "app/build/intermediates/lint.xml", `<lint><issue id="FromBuild" severity="ignore"/></lint>`)

	imp := discoverAndroidLint([]string{root})
	want := []string{"GradleDependency", "IconDensities", "IconMissingDensityFolder", "InvalidPackage", "MissingTranslation", "ObsoleteLintCustomCheck", "Typos", "UnusedResources"}
	if got := imp.IDs(); !reflect.DeepEqual(got, want) {
		t.Errorf("ignored = %v, want %v", got, want)
	}
	if len(imp.Sources) != 3 {
		t.Errorf("sources = %v", imp.Sources)
	}
}

func TestDiscoverAndroidLintNothingFound(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, "build.gradle.kts", `plugins { kotlin("jvm") }`)
	if imp := discoverAndroidLint([]string{root}); imp != nil {
		t.Errorf("import = %+v, want nil", imp)
	}
}

func TestLoadAndroidLintImportHonorsOptOutAndRecordsExplicitActive(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, "lint.xml", `<lint><issue id="MissingTranslation" severity="ignore"/></lint>`)
	writeLintTestFile(t, root, "krit.yml", `
android-lint:
  MissingTranslation:
    active: true
`)
	cfg, err := LoadAndMergeDefaults("", root)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AndroidLint().Ignores("MissingTranslation") {
		t.Error("lint.xml not imported")
	}
	if !cfg.ExplicitlySetsActive("android-lint", "MissingTranslation") {
		t.Error("krit.yml active not recorded as explicit")
	}
	if cfg.ExplicitlySetsActive("android-lint", "ExtraTranslation") {
		t.Error("a shipped default counted as explicit")
	}

	writeLintTestFile(t, root, "krit.yml", "androidLintConfig:\n  enabled: false\n")
	cfg, err = LoadAndMergeDefaults("", root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AndroidLint() != nil {
		t.Error("androidLintConfig.enabled: false did not turn the import off")
	}
}
