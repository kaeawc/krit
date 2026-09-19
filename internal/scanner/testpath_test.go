package scanner

import "testing"

func TestIsTestFileGradleConvention(t *testing.T) {
	InitTestPaths(nil, nil)
	t.Cleanup(func() { InitTestPaths(nil, nil) })

	for _, path := range []string{
		"a/src/commonTest/kotlin/X.kt",
		"a/src/nonWebTest/kotlin/X.kt",
		"a/src/testableTest/kotlin/X.kt",
		"a/src/dockerTest/kotlin/X.kt",
		"a/src/test/java/X.kt",
	} {
		if !IsTestFile(path) {
			t.Errorf("IsTestFile(%q) = false, want true", path)
		}
	}

	for _, path := range []string{
		"a/src/main/kotlin/com/x/TestHelper.kt",
		"a/src/main/kotlin/com/x/Foo.kt",
		"a/src/commonMain/kotlin/X.kt",
	} {
		if IsTestFile(path) {
			t.Errorf("IsTestFile(%q) = true, want false", path)
		}
	}
}

func TestIsTestFileDefaults(t *testing.T) {
	InitTestPaths(nil, nil)
	if !IsTestFile("/repo/app/src/test/kotlin/FooTest.kt") {
		t.Fatal("expected default /test/ path to be test file")
	}
	if IsTestFile("/repo/app/src/main/kotlin/Foo.kt") {
		t.Fatal("expected main source path to be non-test")
	}
}

func TestInitTestPathsAdditive(t *testing.T) {
	InitTestPaths([]string{"/src/checks/"}, nil)
	t.Cleanup(func() { InitTestPaths(nil, nil) })
	if !IsTestFile("/repo/app/src/checks/kotlin/FooCheck.kt") {
		t.Fatal("expected configured path to be test file")
	}
	if !IsTestFile("/repo/app/src/test/kotlin/FooTest.kt") {
		t.Fatal("expected defaults to remain active")
	}
}

func TestInitTestPathsOverride(t *testing.T) {
	InitTestPaths([]string{"/src/checks/"}, []string{"/src/verify/"})
	t.Cleanup(func() { InitTestPaths(nil, nil) })
	if !IsTestFile("/repo/app/src/verify/kotlin/FooVerify.kt") {
		t.Fatal("expected override path to be test file")
	}
	if IsTestFile("/repo/app/src/test/kotlin/FooTest.kt") {
		t.Fatal("expected override to replace defaults")
	}
	if IsTestFile("/repo/app/src/checks/kotlin/FooCheck.kt") {
		t.Fatal("expected override to replace additive config")
	}
}

func TestIsTestFileOverrideSuppressesConvention(t *testing.T) {
	InitTestPaths(nil, []string{"/src/verify/"})
	t.Cleanup(func() { InitTestPaths(nil, nil) })

	path := "a/src/commonTest/kotlin/X.kt"
	if IsTestFile(path) {
		t.Fatal("expected override to suppress Gradle test source set convention")
	}

	InitTestPaths(nil, nil)
	if !IsTestFile(path) {
		t.Fatal("expected Gradle test source set convention after clearing override")
	}
}
