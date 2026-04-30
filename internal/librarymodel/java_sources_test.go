package librarymodel

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestJavaSourceProfile_ConventionalAndroidRootsAndDependencies(t *testing.T) {
	setAndroidHomeWithJar(t, 35)
	root := filepath.Join(t.TempDir(), "app")
	profile := ProfileFromGradleContent(filepath.Join(root, "build.gradle.kts"), `
plugins { id("com.android.application") }

android {
    compileSdk = 35
}

dependencies {
    implementation("androidx.recyclerview:recyclerview:1.3.2")
}
`)
	wantMain := filepath.Join(root, "src", "main", "java")
	wantDebug := filepath.Join(root, "src", "debug", "java")
	wantRelease := filepath.Join(root, "src", "release", "java")
	wantTest := filepath.Join(root, "src", "test", "java")
	wantAndroidTest := filepath.Join(root, "src", "androidTest", "java")
	wantGenerated := filepath.Join(root, "build", "generated", "source")

	for _, want := range []string{wantMain, wantDebug, wantRelease} {
		if !containsString(profile.Java.SourceRoots, want) {
			t.Fatalf("SourceRoots missing %q: %#v", want, profile.Java.SourceRoots)
		}
	}
	if !containsString(profile.Java.TestSourceRoots, wantTest) {
		t.Fatalf("TestSourceRoots missing %q: %#v", wantTest, profile.Java.TestSourceRoots)
	}
	if !containsString(profile.Java.AndroidTestSourceRoots, wantAndroidTest) {
		t.Fatalf("AndroidTestSourceRoots missing %q: %#v", wantAndroidTest, profile.Java.AndroidTestSourceRoots)
	}
	if !containsString(profile.Java.GeneratedSourceRoots, wantGenerated) {
		t.Fatalf("GeneratedSourceRoots missing %q: %#v", wantGenerated, profile.Java.GeneratedSourceRoots)
	}
	if !containsString(profile.Java.ClasspathCandidates, "androidx.recyclerview:recyclerview:1.3.2") {
		t.Fatalf("ClasspathCandidates missing RecyclerView coordinate: %#v", profile.Java.ClasspathCandidates)
	}
	if !profile.Java.ClasspathComplete {
		t.Fatal("ClasspathComplete should be true for direct dependencies and compileSdk")
	}
}

func TestJavaSourceProfile_DeclaredSourceSetsAndScanRoots(t *testing.T) {
	root := filepath.Join(t.TempDir(), "app")
	profile := ProfileFromGradleContent(filepath.Join(root, "build.gradle.kts"), `
plugins { id("com.android.library") }

android {
    compileSdk = 35
    sourceSets {
        getByName("main") {
            java.srcDirs("src/main/java", "src/shared/java")
        }
        getByName("debug") {
            java.srcDir("src/debugOnly/java")
        }
        getByName("test") {
            java.srcDirs("src/testFixtures/java")
        }
        getByName("androidTest") {
            java.srcDirs("src/deviceTest/java")
        }
    }
}
`)
	baseRoots := profile.Java.SourceRootsForScan(false, false, false)
	for _, rel := range []string{"src/main/java", "src/shared/java", "src/debugOnly/java"} {
		want := filepath.Join(root, filepath.FromSlash(rel))
		if !containsString(baseRoots, want) {
			t.Fatalf("base scan roots missing %q: %#v", want, baseRoots)
		}
	}
	for _, rel := range []string{"src/testFixtures/java", "src/deviceTest/java", "build/generated/source"} {
		unwanted := filepath.Join(root, filepath.FromSlash(rel))
		if containsString(baseRoots, unwanted) {
			t.Fatalf("base scan roots should exclude %q: %#v", unwanted, baseRoots)
		}
	}

	allRoots := profile.Java.SourceRootsForScan(true, true, true)
	for _, rel := range []string{"src/testFixtures/java", "src/deviceTest/java", "build/generated/source"} {
		want := filepath.Join(root, filepath.FromSlash(rel))
		if !containsString(allRoots, want) {
			t.Fatalf("expanded scan roots missing %q: %#v", want, allRoots)
		}
	}
}

func TestJavaSourceProfile_CatalogDependenciesUpdateClasspathCandidates(t *testing.T) {
	setAndroidHomeWithJar(t, 35)
	root := filepath.Join(t.TempDir(), "app")
	catalog := ParseVersionCatalogContent(`
[versions]
okhttp = "4.12.0"

[libraries]
okhttp = { group = "com.squareup.okhttp3", name = "okhttp", version.ref = "okhttp" }
`)
	profile := ProfileFromGradleContentWithCatalog(filepath.Join(root, "build.gradle.kts"), `
plugins { id("com.android.library") }

android {
    compileSdk = 35
}

dependencies {
    implementation(libs.okhttp)
}
`, catalog)
	if !containsString(profile.Java.ClasspathCandidates, "com.squareup.okhttp3:okhttp:4.12.0") {
		t.Fatalf("ClasspathCandidates missing catalog-resolved dependency: %#v", profile.Java.ClasspathCandidates)
	}
	if !profile.Java.ClasspathComplete {
		t.Fatal("ClasspathComplete should stay true when catalog aliases resolve")
	}
}

func TestJavaSourceProfile_IncompleteWhenDependenciesUnresolved(t *testing.T) {
	profile := ProfileFromGradleContent("build.gradle.kts", `
plugins { id("com.android.library") }

android {
    compileSdk = 35
}

dependencies {
    implementation(libs.missing.alias)
}
`)
	if !profile.HasUnresolvedDependencyRefs {
		t.Fatal("expected unresolved dependency refs")
	}
	if profile.Java.ClasspathComplete {
		t.Fatal("Java classpath should be incomplete when dependency refs are unresolved")
	}
}

func TestJavaSourceProfile_JavacClasspathUsesAvailableAndroidJar(t *testing.T) {
	jar := setAndroidHomeWithJar(t, 35)
	profile := ProfileFromGradleContent("build.gradle.kts", `
plugins { id("com.android.application") }

android {
    compileSdk = 35
}
`)
	if got := profile.Java.JavacClasspathCandidates(); !reflect.DeepEqual(got, []string{jar}) {
		t.Fatalf("JavacClasspathCandidates = %#v, want %#v", got, []string{jar})
	}
	if got := profile.Java.JavacClasspathArg(); !strings.Contains(got, jar) {
		t.Fatalf("JavacClasspathArg = %q, want it to contain %q", got, jar)
	}
}

func setAndroidHomeWithJar(t *testing.T, compileSDK int) string {
	t.Helper()
	sdk := t.TempDir()
	jar := filepath.Join(sdk, "platforms", "android-"+strconv.Itoa(compileSDK), "android.jar")
	if err := os.MkdirAll(filepath.Dir(jar), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jar, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANDROID_HOME", sdk)
	t.Setenv("ANDROID_SDK_ROOT", "")
	return jar
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
