package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/fixer"
	"github.com/kaeawc/krit/internal/module"
	rulespkg "github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestCompileSdkMismatchAcrossModules(t *testing.T) {
	registered := buildRuleIndex()["CompileSdkMismatchAcrossModules"]
	if registered == nil {
		t.Fatal("CompileSdkMismatchAcrossModules rule not registered")
	}

	root := fixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "supply-chain", "compile-sdk-mismatch-across-modules")
	negativeDir := filepath.Join(root, "negative", "supply-chain", "compile-sdk-mismatch-across-modules")

	t.Run("positive fixture triggers", func(t *testing.T) {
		findings := runCompileSdkMismatchAcrossModulesRule(t, positiveDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, ":feature:a=33") || !strings.Contains(findings[0].Message, ":feature:b=34") {
			t.Fatalf("expected finding to summarize both module compileSdk values, got %q", findings[0].Message)
		}
		if !strings.Contains(findings[0].Message, "Module :feature:a declares compileSdk 33") {
			t.Fatalf("expected finding to point at the lower compileSdk module, got %q", findings[0].Message)
		}
		if !strings.HasSuffix(filepath.ToSlash(findings[0].File), "/feature/a/build.gradle.kts") {
			t.Fatalf("expected finding to point at feature/a build file, got %q", findings[0].File)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		findings := runCompileSdkMismatchAcrossModulesRule(t, negativeDir)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func runCompileSdkMismatchAcrossModulesRule(t *testing.T, projectDir string) []scanner.Finding {
	t.Helper()

	graph, err := module.DiscoverModules(projectDir)
	if err != nil {
		t.Fatalf("DiscoverModules(%s): %v", projectDir, err)
	}
	if graph == nil {
		t.Fatalf("expected modules to be discovered in %s", projectDir)
	}

	rule := buildRuleIndex()["CompileSdkMismatchAcrossModules"]
	if rule == nil {
		t.Fatal("CompileSdkMismatchAcrossModules not registered")
	}

	ctx := &api.Context{
		ModuleIndex: &module.PerModuleIndex{Graph: graph},
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.Check(ctx)
	return api.ContextFindings(ctx)
}

func TestGradleWrapperValidationAction(t *testing.T) {
	r := buildRuleIndex()["GradleWrapperValidationAction"]
	if r == nil {
		t.Fatal("GradleWrapperValidationAction rule not registered")
	}

	t.Run("gradle action without preceding validation triggers", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, ".github", "workflows", "ci.yml"), `jobs:
  build:
    steps:
      - uses: actions/checkout@v4
      - uses: gradle/gradle-build-action@v2
  setup:
    steps:
      - uses: gradle/wrapper-validation-action@v2
      - uses: gradle/actions/setup-gradle@v4
`)

		findings := runSupplyModuleRule(t, r, root)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 5 {
			t.Fatalf("expected line 5, got %d", findings[0].Line)
		}
	})

	t.Run("validation in different job does not protect build job", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, ".github", "workflows", "ci.yaml"), `jobs:
  validate:
    steps:
      - uses: gradle/wrapper-validation-action@v2
  build:
    steps:
      - uses: gradle/actions/setup-gradle@v4
`)

		findings := runSupplyModuleRule(t, r, root)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("preceding validation and non-gradle workflows are clean", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, ".github", "workflows", "ci.yml"), `jobs:
  build:
    steps:
      - uses: gradle/actions/wrapper-validation@v3
      - uses: gradle/actions/setup-gradle@v4
  docs:
    steps:
      - run: echo no gradle action
`)

		findings := runSupplyModuleRule(t, r, root)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestJvmTargetMismatch(t *testing.T) {
	r := findGradleRule(t, "JvmTargetMismatch")

	t.Run("kotlin and java target mismatch triggers", func(t *testing.T) {
		content := `android {
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
    }
    kotlinOptions {
        jvmTarget = "17"
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 6 {
			t.Fatalf("expected line 6, got %d", findings[0].Line)
		}
	})

	t.Run("toolchain and explicit override mismatch triggers", func(t *testing.T) {
		content := `kotlin {
    jvmToolchain(17)
}
tasks.withType<org.jetbrains.kotlin.gradle.tasks.KotlinCompile>().configureEach {
    kotlinOptions.jvmTarget = "11"
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("matching targets and toolchain alone are clean", func(t *testing.T) {
		content := `android {
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
}
kotlin {
    jvmToolchain(17)
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}

		toolchainOnly := `kotlin {
    jvmToolchain {
        languageVersion = JavaLanguageVersion.of(17)
    }
}
`
		cfg, _ = android.ParseBuildGradleContent(toolchainOnly)
		findings = runGradleRule(r, "build.gradle.kts", toolchainOnly, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for toolchain only, got %d", len(findings))
		}
	})
}

func TestKotlinVersionMismatchAcrossModules(t *testing.T) {
	r := buildRuleIndex()["KotlinVersionMismatchAcrossModules"]
	if r == nil {
		t.Fatal("KotlinVersionMismatchAcrossModules rule not registered")
	}

	t.Run("module target mismatch triggers minority module", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "a", "build.gradle.kts"), `kotlin { jvmToolchain(17) }`)
		writeFile(t, filepath.Join(root, "b", "build.gradle.kts"), `kotlin { jvmToolchain(17) }`)
		writeFile(t, filepath.Join(root, "c", "build.gradle.kts"), `kotlin { jvmToolchain(11) }`)

		findings := runKotlinVersionMismatchAcrossModules(t, r, root)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "Module :c uses Kotlin JVM target 11") {
			t.Fatalf("expected :c minority module finding, got %q", findings[0].Message)
		}
	})

	t.Run("matching modules and single configured module are clean", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "a", "build.gradle.kts"), `tasks.withType<org.jetbrains.kotlin.gradle.tasks.KotlinCompile>().configureEach {
    kotlinOptions.jvmTarget = "17"
}`)
		writeFile(t, filepath.Join(root, "b", "build.gradle.kts"), `kotlin { jvmToolchain(17) }`)

		findings := runKotlinVersionMismatchAcrossModules(t, r, root)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}

		single := t.TempDir()
		writeFile(t, filepath.Join(single, "a", "build.gradle.kts"), `kotlin { jvmToolchain(11) }`)
		findings = runKotlinVersionMismatchAcrossModules(t, r, single)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for single module, got %d", len(findings))
		}
	})
}

func runKotlinVersionMismatchAcrossModules(t *testing.T, r *api.Rule, root string) []scanner.Finding {
	t.Helper()
	graph := module.NewModuleGraph(root)
	for _, name := range []string{"a", "b", "c"} {
		dir := filepath.Join(root, name)
		if _, err := os.Stat(dir); err == nil {
			graph.Modules[":"+name] = &module.Module{Path: ":" + name, Dir: dir}
		}
	}
	ctx := &api.Context{
		ModuleIndex: &module.PerModuleIndex{Graph: graph},
		Collector:   scanner.NewFindingCollector(0),
	}
	r.Check(ctx)
	return api.ContextFindings(ctx)
}

func TestDependencyVerificationDisabled(t *testing.T) {
	r := buildRuleIndex()["DependencyVerificationDisabled"]
	if r == nil {
		t.Fatal("DependencyVerificationDisabled rule not registered")
	}

	t.Run("off and lenient trigger", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "gradle.properties"), `
# org.gradle.dependency.verification=strict
org.gradle.dependency.verification=off
`)
		writeFile(t, filepath.Join(root, "app", "gradle.properties"), `
org.gradle.dependency.verification: lenient
`)

		findings := runDependencyVerificationDisabled(t, r, root)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("strict comments and unrelated properties are clean", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "gradle.properties"), `
# org.gradle.dependency.verification=off
! org.gradle.dependency.verification=lenient
org.gradle.dependency.verification=strict
org.gradle.dependency.verification.mode=off
`)

		findings := runDependencyVerificationDisabled(t, r, root)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("allowLenient still flags off", func(t *testing.T) {
		impl := r.Implementation.(*rulespkg.DependencyVerificationDisabledRule)
		oldAllowLenient := impl.AllowLenient
		impl.AllowLenient = true
		t.Cleanup(func() { impl.AllowLenient = oldAllowLenient })

		root := t.TempDir()
		writeFile(t, filepath.Join(root, "gradle.properties"), `
org.gradle.dependency.verification=lenient
`)
		writeFile(t, filepath.Join(root, "gradle", "gradle.properties"), `
org.gradle.dependency.verification=off
`)

		findings := runDependencyVerificationDisabled(t, r, root)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, `"off"`) {
			t.Fatalf("expected off finding, got %q", findings[0].Message)
		}
	})
}

func TestMissingGradleChecksums(t *testing.T) {
	r := buildRuleIndex()["MissingGradleChecksums"]
	if r == nil {
		t.Fatal("MissingGradleChecksums rule not registered")
	}

	t.Run("dependency locking without lockfile triggers", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "settings.gradle.kts"), `dependencyLocking {
    lockAllConfigurations()
}
`)

		findings := runSupplyModuleRule(t, r, root)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 1 {
			t.Fatalf("expected line 1, got %d", findings[0].Line)
		}
	})

	t.Run("lockfile present and no locking are clean", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "build.gradle.kts"), `dependencyLocking {
    lockAllConfigurations()
}
`)
		writeFile(t, filepath.Join(root, "gradle.lockfile"), `empty=lockfile
`)
		writeFile(t, filepath.Join(root, "app", "build.gradle.kts"), `plugins {
    kotlin("jvm")
}
`)

		findings := runSupplyModuleRule(t, r, root)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("module lockfile checked in same directory", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "app", "build.gradle.kts"), `dependencyLocking {
    lockMode = LockMode.STRICT
}
`)

		findings := runSupplyModuleRule(t, r, root)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.HasSuffix(filepath.ToSlash(findings[0].File), "/app/build.gradle.kts") {
			t.Fatalf("expected app build file, got %q", findings[0].File)
		}
	})
}

func runDependencyVerificationDisabled(t *testing.T, r *api.Rule, root string) []scanner.Finding {
	t.Helper()
	return runSupplyModuleRule(t, r, root)
}

func runSupplyModuleRule(t *testing.T, r *api.Rule, root string) []scanner.Finding {
	t.Helper()
	graph := module.NewModuleGraph(root)
	graph.Modules[":app"] = &module.Module{Path: ":app", Dir: filepath.Join(root, "app")}
	ctx := &api.Context{
		ModuleIndex: &module.PerModuleIndex{Graph: graph},
		Collector:   scanner.NewFindingCollector(0),
	}
	r.Check(ctx)
	return api.ContextFindings(ctx)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// applySuggestionEdits applies the edits of the named suggested fix on the
// given finding through the same fixer pipeline used by `krit apply-suggestion`.
// It fails the test when the suggestion or its edits are missing or do not
// apply cleanly.
func applySuggestionEdits(t *testing.T, finding scanner.Finding, suggestionID string) {
	t.Helper()
	var sug *scanner.SuggestedFix
	for i := range finding.SuggestedFixes {
		if finding.SuggestedFixes[i].ID == suggestionID {
			sug = &finding.SuggestedFixes[i]
			break
		}
	}
	if sug == nil {
		t.Fatalf("suggestion %q not found on finding", suggestionID)
	}
	if len(sug.Edits) == 0 {
		t.Fatalf("suggestion %q has no edits to apply", suggestionID)
	}
	edits := make([]scanner.Finding, 0, len(sug.Edits))
	for _, edit := range sug.Edits {
		edits = append(edits, scanner.Finding{
			File:     finding.File,
			Line:     finding.Line,
			Col:      finding.Col,
			RuleSet:  finding.RuleSet,
			Rule:     finding.Rule,
			Severity: finding.Severity,
			Message:  finding.Message,
			Fix: &scanner.Fix{
				TargetFile:  edit.TargetFile,
				StartLine:   edit.StartLine,
				EndLine:     edit.EndLine,
				StartByte:   edit.StartByte,
				EndByte:     edit.EndByte,
				ByteMode:    edit.ByteMode,
				Replacement: edit.Replacement,
			},
		})
	}
	columns := scanner.CollectFindings(edits)
	applied, _, errs := fixer.ApplyAllFixesColumns(&columns, "")
	if len(errs) > 0 {
		t.Fatalf("ApplyAllFixesColumns errors: %v", errs)
	}
	if applied != len(sug.Edits) {
		t.Fatalf("applied fixes = %d, want %d", applied, len(sug.Edits))
	}
}

func TestDependencySnapshotInRelease(t *testing.T) {
	r := findGradleRule(t, "DependencySnapshotInRelease")

	t.Run("string and named arg snapshot dependencies trigger", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:lib:1.2.3-SNAPSHOT")
    api group: "org.example", name: "core", version: "2.0.0-SNAPSHOT"
    releaseImplementation(group = "net.example", name = "release-lib", version = "3.0.0-SNAPSHOT")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}
		if findings[0].Line != 2 {
			t.Fatalf("expected first finding on line 2, got %d", findings[0].Line)
		}
	})

	t.Run("pinned versions comments and unrelated strings are clean", func(t *testing.T) {
		content := `val docs = "com.example:lib:1.2.3-SNAPSHOT"
dependencies {
    // implementation("com.example:lib:1.2.3-SNAPSHOT")
    implementation("com.example:lib:1.2.3")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("allow list suppresses matching coordinates", func(t *testing.T) {
		impl := r.Implementation.(*rulespkg.DependencySnapshotInReleaseRule)
		oldAllowed := impl.AllowedSnapshots
		impl.AllowedSnapshots = []string{"com.example:lib", "org.corp.internal:*"}
		t.Cleanup(func() { impl.AllowedSnapshots = oldAllowed })

		content := `dependencies {
    implementation("com.example:lib:1.2.3-SNAPSHOT")
    implementation("org.corp.internal:tooling:1.0.0-SNAPSHOT")
    implementation("org.other:lib:1.0.0-SNAPSHOT")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "org.other:lib") {
			t.Fatalf("expected unallowlisted coordinate in finding, got %q", findings[0].Message)
		}
	})

	t.Run("future suppressUntil suppresses rule", func(t *testing.T) {
		impl := r.Implementation.(*rulespkg.DependencySnapshotInReleaseRule)
		oldUntil := impl.SuppressUntil
		impl.SuppressUntil = "2999-01-01"
		t.Cleanup(func() { impl.SuppressUntil = oldUntil })

		content := `dependencies {
    implementation("com.example:lib:1.2.3-SNAPSHOT")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestDependenciesInRootProject(t *testing.T) {
	r := findGradleRule(t, "DependenciesInRootProject")

	t.Run("rule advertises suggested fixes and no autofix", func(t *testing.T) {
		if r.Fix != api.FixNone {
			t.Fatalf("Fix = %v, want FixNone (rule must not advertise an autofix)", r.Fix)
		}
		if mode := r.FixMode(); mode != api.FixModeSuggested {
			t.Fatalf("FixMode = %v, want FixModeSuggested", mode)
		}
		if len(r.SuggestedFixes) != 2 {
			t.Fatalf("SuggestedFixes count = %d, want 2", len(r.SuggestedFixes))
		}
		wantIDs := []string{
			rulespkg.DependenciesInRootProjectMoveSuggestionID,
			rulespkg.DependenciesInRootProjectAllowSuggestionID,
		}
		for i, want := range wantIDs {
			if got := r.SuggestedFixes[i].ID; got != want {
				t.Errorf("SuggestedFixes[%d].ID = %q, want %q", i, got, want)
			}
		}
		if err := r.ValidateFixMode(); err != nil {
			t.Fatalf("ValidateFixMode: %v", err)
		}
	})

	t.Run("root build dependencies emit ordered suggestions", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "settings.gradle.kts"), `pluginManagement { repositories { gradlePluginPortal() } }`)
		buildPath := filepath.Join(root, "build.gradle.kts")
		content := `plugins {
    kotlin("jvm") version "2.0.0" apply false
}

dependencies {
    implementation("com.example:lib:1.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, buildPath, content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "implementation") {
			t.Fatalf("expected message to name implementation, got %q", findings[0].Message)
		}
		if findings[0].Fix != nil {
			t.Fatal("finding must not carry an autofix when SuggestedFixes are emitted")
		}
		got := findings[0].SuggestedFixes
		if len(got) != 2 {
			t.Fatalf("expected 2 suggested fixes, got %d", len(got))
		}
		if got[0].ID != rulespkg.DependenciesInRootProjectMoveSuggestionID {
			t.Errorf("suggestion[0].ID = %q, want %q", got[0].ID, rulespkg.DependenciesInRootProjectMoveSuggestionID)
		}
		if len(got[0].Edits) != 0 {
			t.Errorf("moveToOwningModule must be guidance-only (no edits), got %d", len(got[0].Edits))
		}
		if got[1].ID != rulespkg.DependenciesInRootProjectAllowSuggestionID {
			t.Errorf("suggestion[1].ID = %q, want %q", got[1].ID, rulespkg.DependenciesInRootProjectAllowSuggestionID)
		}
		if len(got[1].Edits) != 1 {
			t.Fatalf("addAllowedConfigurations must carry exactly 1 edit, got %d", len(got[1].Edits))
		}
		if want := filepath.Join(root, "krit.yml"); got[1].Edits[0].TargetFile != want {
			t.Errorf("edit.TargetFile = %q, want %q", got[1].Edits[0].TargetFile, want)
		}
	})

	t.Run("applying allow suggestion writes root krit config allowlist", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "settings.gradle.kts"), ``)
		buildPath := filepath.Join(root, "build.gradle.kts")
		content := `dependencies {
    implementation("com.example:lib:1.0")
    ksp("com.example:processor:1.0")
    testImplementation("com.example:test:1.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, buildPath, content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		applySuggestionEdits(t, findings[0], rulespkg.DependenciesInRootProjectAllowSuggestionID)

		got := readFile(t, filepath.Join(root, "krit.yml"))
		for _, want := range []string{"classpath", "detektPlugins", "implementation", "ksp", "testImplementation"} {
			if !strings.Contains(got, "- "+want) {
				t.Fatalf("krit.yml missing %q:\n%s", want, got)
			}
		}
	})

	t.Run("applying allow suggestion preserves generated krit config header", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "settings.gradle.kts"), ``)
		writeFile(t, filepath.Join(root, "krit.yml"), `# Generated by krit init (bubbletea TUI)
# Profile: balanced

style:
    MagicNumber:
        active: false
`)
		buildPath := filepath.Join(root, "build.gradle.kts")
		content := `dependencies {
    implementation("com.example:lib:1.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, buildPath, content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		applySuggestionEdits(t, findings[0], rulespkg.DependenciesInRootProjectAllowSuggestionID)

		got := readFile(t, filepath.Join(root, "krit.yml"))
		if !strings.HasPrefix(got, "# Generated by krit init (bubbletea TUI)\n# Profile: balanced\n\n") {
			t.Fatalf("krit.yml header not preserved:\n%s", got)
		}
		if !strings.Contains(got, "allowedConfigurations:") {
			t.Fatalf("krit.yml missing allowedConfigurations:\n%s", got)
		}
	})

	t.Run("allowed root tooling configurations are clean", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "settings.gradle.kts"), ``)
		buildPath := filepath.Join(root, "build.gradle")
		content := `buildscript {
    dependencies {
        classpath "com.android.tools.build:gradle:8.6.0"
    }
}

dependencies {
    detektPlugins("io.gitlab.arturbosch.detekt:detekt-formatting:1.23.7")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, buildPath, content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("subprojects and module dependencies are clean", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "settings.gradle.kts"), `include(":app")`)
		rootBuildPath := filepath.Join(root, "build.gradle.kts")
		rootBuild := `subprojects {
    dependencies {
        implementation("com.example:lib:1.0")
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(rootBuild)
		findings := runGradleRule(r, rootBuildPath, rootBuild, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for subprojects block, got %d", len(findings))
		}

		moduleBuildPath := filepath.Join(root, "app", "build.gradle.kts")
		moduleBuild := `dependencies {
    implementation("com.example:lib:1.0")
}
`
		cfg, _ = android.ParseBuildGradleContent(moduleBuild)
		findings = runGradleRule(r, moduleBuildPath, moduleBuild, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for module build file, got %d", len(findings))
		}
	})
}

func TestDependencyWithoutGroup(t *testing.T) {
	r := findGradleRule(t, "DependencyWithoutGroup")

	t.Run("two-part dependency coordinates trigger", func(t *testing.T) {
		content := `dependencies {
    testImplementation("junit:4.13")
    compile 'legacy:1.0'
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
		if findings[0].Line != 2 {
			t.Fatalf("expected first finding on line 2, got %d", findings[0].Line)
		}
	})

	t.Run("full coordinates maps comments projects and files are clean", func(t *testing.T) {
		content := `val docs = "junit:4.13"
dependencies {
    // testImplementation("junit:4.13")
    testImplementation("junit:junit:4.13")
    implementation(project(":foo"))
    implementation(files("libs/foo.jar"))
    api group: "junit", name: "junit", version: "4.13"
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestApplyPluginTwice(t *testing.T) {
	r := findGradleRule(t, "ApplyPluginTwice")

	t.Run("fixtures", func(t *testing.T) {
		root := fixtureRoot(t)
		for _, tc := range []struct {
			name string
			path string
			want int
		}{
			{
				name: "positive",
				path: filepath.Join(root, "positive", "supply-chain", "apply-plugin-twice", "build.gradle.kts"),
				want: 1,
			},
			{
				name: "negative",
				path: filepath.Join(root, "negative", "supply-chain", "apply-plugin-twice", "build.gradle.kts"),
				want: 0,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				data, err := os.ReadFile(tc.path)
				if err != nil {
					t.Fatal(err)
				}
				content := string(data)
				cfg, _ := android.ParseBuildGradleContent(content)
				findings := runGradleRule(r, tc.path, content, cfg)
				if len(findings) != tc.want {
					t.Fatalf("expected %d findings, got %d", tc.want, len(findings))
				}
			})
		}
	})

	t.Run("kotlin dsl duplicate triggers", func(t *testing.T) {
		content := `plugins {
    id("com.android.application")
}

apply(plugin = "com.android.application")
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 5 {
			t.Fatalf("expected finding on apply line 5, got %d", findings[0].Line)
		}
		if !strings.Contains(findings[0].Message, "com.android.application") {
			t.Fatalf("expected duplicate plugin id in message, got %q", findings[0].Message)
		}
	})

	t.Run("groovy apply and kotlin plugin alias normalize", func(t *testing.T) {
		content := `plugins {
    kotlin("jvm")
}

apply plugin: "org.jetbrains.kotlin.jvm"
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "org.jetbrains.kotlin.jvm") {
			t.Fatalf("expected normalized Kotlin plugin id in message, got %q", findings[0].Message)
		}
	})

	t.Run("either form alone and different ids are clean", func(t *testing.T) {
		cases := []string{
			`plugins { id("com.android.application") }`,
			`apply(plugin = "com.android.application")`,
			`plugins { id("com.android.application") }
apply(plugin = "com.example.convention.android")`,
		}
		for _, content := range cases {
			cfg, _ := android.ParseBuildGradleContent(content)
			findings := runGradleRule(r, "build.gradle.kts", content, cfg)
			if len(findings) != 0 {
				t.Fatalf("expected 0 findings for %q, got %d", content, len(findings))
			}
		}
	})

	t.Run("apply false strings comments and settings are clean", func(t *testing.T) {
		content := `val docs = "apply(plugin = \"com.android.application\")"
plugins {
    id("com.android.application") apply false
}
// apply(plugin = "com.android.application")
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}

		settingsFindings := runGradleRule(r, "settings.gradle.kts", `plugins { id("com.android.application") }
apply(plugin = "com.android.application")
`, cfg)
		if len(settingsFindings) != 0 {
			t.Fatalf("expected 0 findings for settings file, got %d", len(settingsFindings))
		}
	})

	t.Run("fix removes duplicate apply line", func(t *testing.T) {
		content := `plugins {
    id("com.android.application")
}

apply(plugin = "com.android.application")
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		fix := findings[0].Fix
		if fix == nil {
			t.Fatalf("expected fix on finding, got nil")
		}
		if !fix.ByteMode {
			t.Fatalf("expected byte-mode fix")
		}
		got := content[:fix.StartByte] + fix.Replacement + content[fix.EndByte:]
		want := `plugins {
    id("com.android.application")
}

`
		if got != want {
			t.Fatalf("fix output mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
		}
	})
}

func TestConfigurationsAllSideEffect(t *testing.T) {
	r := findGradleRule(t, "ConfigurationsAllSideEffect")

	t.Run("configurations all mutator triggers", func(t *testing.T) {
		content := `plugins { kotlin("jvm") version "1.9.24" }

configurations.all {
    resolutionStrategy.force("com.squareup.okhttp3:okhttp:4.12.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 3 {
			t.Fatalf("expected finding on line 3, got %d", findings[0].Line)
		}
		if !strings.Contains(findings[0].Message, "resolutionStrategy") {
			t.Fatalf("expected mutator in message, got %q", findings[0].Message)
		}
	})

	t.Run("matching all is clean", func(t *testing.T) {
		content := `plugins { kotlin("jvm") version "1.9.24" }

configurations.matching { it.name == "runtimeClasspath" }.all {
    resolutionStrategy.force("com.squareup.okhttp3:okhttp:4.12.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("read only block strings and comments are clean", func(t *testing.T) {
		content := `val docs = "configurations.all { resolutionStrategy.force(\"x:y:1\") }"
// configurations.all { resolutionStrategy.force("x:y:1") }
configurations.all {
    println(name)
    println(resolutionStrategy)
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("convention plugin paths are configurable", func(t *testing.T) {
		impl := r.Implementation.(*rulespkg.ConfigurationsAllSideEffectRule)
		original := impl.AllowInConventionPlugins
		defer func() { impl.AllowInConventionPlugins = original }()

		content := `configurations.all {
    resolutionStrategy {
        force("com.squareup.okhttp3:okhttp:4.12.0")
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		path := filepath.Join("repo", "build-logic", "build.gradle.kts")
		relativePath := filepath.Join("buildSrc", "build.gradle.kts")

		impl.AllowInConventionPlugins = true
		findings := runGradleRule(r, path, content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for allowed convention plugin path, got %d", len(findings))
		}
		findings = runGradleRule(r, relativePath, content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for allowed relative convention plugin path, got %d", len(findings))
		}

		impl.AllowInConventionPlugins = false
		findings = runGradleRule(r, path, content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding when convention plugin allowance is disabled, got %d", len(findings))
		}
	})
}

func TestDependencyFromBintray(t *testing.T) {
	r := findGradleRule(t, "DependencyFromBintray")

	t.Run("bintray repository urls trigger", func(t *testing.T) {
		content := `repositories {
    maven { url = uri("https://dl.bintray.com/example/maven") }
    maven("https://jcenter.bintray.com")
    ivy { url "https://plugins.bintray.com/gradle" }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}
		if findings[0].Line != 2 {
			t.Fatalf("expected first finding on line 2, got %d", findings[0].Line)
		}
	})

	t.Run("safe repositories and comments are clean", func(t *testing.T) {
		content := `val docs = "https://dl.bintray.com/example/maven"
repositories {
    // maven { url = uri("https://dl.bintray.com/example/maven") }
    mavenCentral()
    maven { url = uri("https://repo.example.com/maven") }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestDependencyFromJcenter(t *testing.T) {
	r := findGradleRule(t, "DependencyFromJcenter")

	t.Run("build repositories block triggers", func(t *testing.T) {
		content := `repositories {
    mavenCentral()
    jcenter()
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 3 {
			t.Fatalf("expected line 3, got %d", findings[0].Line)
		}
	})

	t.Run("buildscript and settings repositories trigger", func(t *testing.T) {
		content := `buildscript {
    repositories { jcenter() }
}
dependencyResolutionManagement {
    repositories {
        jcenter {
            content { includeGroup("legacy") }
        }
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "settings.gradle.kts", content, cfg)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("safe repositories comments and lookalikes are clean", func(t *testing.T) {
		content := `// repositories { jcenter() }
val docs = "repositories { jcenter() }"
fun jcenter() = Unit
repositories {
    google()
    mavenCentral()
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestDependencyFromHttp(t *testing.T) {
	r := findGradleRule(t, "DependencyFromHttp")

	t.Run("maven url assignment triggers", func(t *testing.T) {
		content := `dependencyResolutionManagement {
    repositories {
        google()
        maven { url = uri("http://repo.example.com/maven") }
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "settings.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 4 {
			t.Fatalf("expected line 4, got %d", findings[0].Line)
		}
	})

	t.Run("maven shorthand and ivy url trigger", func(t *testing.T) {
		content := `repositories {
    maven("http://repo.example.com/maven")
    ivy {
        url "http://repo.example.com/ivy"
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("https comments and unrelated strings are clean", func(t *testing.T) {
		content := `val docs = "http://repo.example.com/maven"
repositories {
    // maven { url = uri("http://repo.example.com/maven") }
    maven { url = uri("https://repo.example.com/maven") }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("loopback is allowed by default", func(t *testing.T) {
		content := `repositories {
    maven { url = uri("http://localhost:8081/repository") }
    maven { url = uri("http://127.0.0.1:8081/repository") }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("configured host and url prefix are allowed", func(t *testing.T) {
		impl := r.Implementation.(*rulespkg.DependencyFromHTTPRule)
		oldHosts, oldURLs := impl.AllowedHosts, impl.AllowedUrls
		impl.AllowedHosts = []string{"mirror.internal"}
		impl.AllowedUrls = []string{"http://repo.example.com/allowed"}
		t.Cleanup(func() {
			impl.AllowedHosts = oldHosts
			impl.AllowedUrls = oldURLs
		})

		content := `repositories {
    maven { url = uri("http://mirror.internal/maven") }
    maven { url = uri("http://repo.example.com/allowed/team") }
    maven { url = uri("http://repo.example.com/blocked") }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "http://repo.example.com/blocked") {
			t.Fatalf("expected blocked URL in finding, got %q", findings[0].Message)
		}
	})
}
