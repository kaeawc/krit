package rules_test

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func testManifestPositions(t *testing.T, path, content string) []string {
	t.Helper()
	var out []string
	for _, f := range runLintGradleRule(t, "TestManifestGradleConfiguration", path, content) {
		if f.Message != "Please use debugImplementation." {
			t.Errorf("unexpected message %q", f.Message)
		}
		out = append(out, fmt.Sprintf("%d:%d", f.Line, f.Col))
	}
	sort.Slice(out, func(i, j int) bool {
		var li, lj int
		fmt.Sscanf(out[i], "%d", &li)
		fmt.Sscanf(out[j], "%d", &lj)
		return li < lj
	})
	return out
}

// Expected positions come from running the lint.jar of androidx.compose.ui:ui-test-manifest 1.12.1 through the
// Android Lint CLI on the same Groovy build file. Lint matches any statement whose name contains a configuration
// and starts with one (or with test/android), in any block, so the ext assignment and the testImplementation
// inside android.defaultConfig are reported too; closure-form, platform(), map and catalog declarations are not.
func TestTestManifestGradleConfigurationLintParity(t *testing.T) {
	content := `dependencies {
    implementation 'androidx.compose.ui:ui-test-manifest:1.12.1'
    debugImplementation 'androidx.compose.ui:ui-test-manifest:1.12.1'
    testImplementation "androidx.compose.ui:ui-test-manifest:1.12.1"
    androidTestImplementation('androidx.compose.ui:ui-test-manifest:1.12.1')
    releaseImplementation 'androidx.compose.ui:ui-test-manifest:1.12.1'
    api "androidx.compose.ui:ui-test-manifest"
    compileOnly 'androidx.compose.ui:ui-test-manifest:1.12.1'
    runtimeOnly 'androidx.compose.ui:ui-test-manifest:1.12.1'
    lintChecks 'androidx.compose.ui:ui-test-manifest:1.12.1'
    annotationProcessor 'androidx.compose.ui:ui-test-manifest:1.12.1'
    kapt 'androidx.compose.ui:ui-test-manifest:1.12.1'
    debugApi 'androidx.compose.ui:ui-test-manifest:1.12.1'
    testDebugImplementation 'androidx.compose.ui:ui-test-manifest:1.12.1'
    androidTestDebugImplementation 'androidx.compose.ui:ui-test-manifest:1.12.1'
    implementation libs.androidx.compose.ui.test.manifest
    implementation platform('androidx.compose.ui:ui-test-manifest:1.12.1')
    implementation('androidx.compose.ui:ui-test-manifest:1.12.1') {
        exclude group: 'x'
    }
    implementation 'androidx.compose.ui:ui-test-junit4:1.12.1'
    implementation 'androidx.compose.ui:ui-test-manifest-extra:1.0'
    implementation group: 'androidx.compose.ui', name: 'ui-test-manifest', version: '1.12.1'
    // implementation 'androidx.compose.ui:ui-test-manifest:1.12.1'
    freeImplementation 'androidx.compose.ui:ui-test-manifest:1.12.1'
    testFixturesImplementation 'androidx.compose.ui:ui-test-manifest:1.12.1'
}
ext {
    implementationOfManifest = 'androidx.compose.ui:ui-test-manifest:1.12.1'
}
android {
    defaultConfig {
        testImplementation 'androidx.compose.ui:ui-test-manifest:1.12.1'
    }
}
`
	want := []string{
		"2:5",
		"4:5",
		"5:5",
		"7:5",
		"8:5",
		"9:5",
		"10:5",
		"11:5",
		"22:5",
		"26:5",
		"29:5",
		"33:9",
	}
	if got := testManifestPositions(t, "build.gradle", content); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The Lint CLI cannot run Gradle checks on Kotlin build scripts, so these follow the detector's source.
func TestTestManifestGradleConfigurationKotlinDSL(t *testing.T) {
	content := `dependencies {
    implementation("androidx.compose.ui:ui-test-manifest:1.12.1")
    debugImplementation("androidx.compose.ui:ui-test-manifest:1.12.1")
    androidTestImplementation("androidx.compose.ui:ui-test-manifest")
    implementation(libs.androidx.compose.ui.test.manifest)
    "implementation"("androidx.compose.ui:ui-test-manifest:1.12.1")
}
`
	want := []string{"2:5", "4:5"}
	if got := testManifestPositions(t, "build.gradle.kts", content); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Lint's quick fix replaces the configuration name with debugImplementation.
func TestTestManifestGradleConfigurationFix(t *testing.T) {
	content := "dependencies {\n    testImplementation 'androidx.compose.ui:ui-test-manifest:1.12.1'\n}\n"
	findings := runLintGradleRule(t, "TestManifestGradleConfiguration", "build.gradle", content)
	if len(findings) != 1 || findings[0].Fix == nil {
		t.Fatalf("want one finding with a fix, got %+v", findings)
	}
	fix := findings[0].Fix
	got := content[:fix.StartByte] + fix.Replacement + content[fix.EndByte:]
	if want := strings.Replace(content, "testImplementation", "debugImplementation", 1); got != want {
		t.Fatalf("fixed content %q, want %q", got, want)
	}
}

func TestTestManifestGradleConfigurationIgnoresComments(t *testing.T) {
	content := `dependencies {
    // implementation 'androidx.compose.ui:ui-test-manifest:1.12.1'
    /* implementation 'androidx.compose.ui:ui-test-manifest:1.12.1' */
    debugImplementation 'androidx.compose.ui:ui-test-manifest:1.12.1'
}
`
	if got := testManifestPositions(t, "build.gradle", content); len(got) != 0 {
		t.Fatalf("got %q, want none", got)
	}
}
