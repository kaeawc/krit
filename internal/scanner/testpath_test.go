package scanner

import "testing"

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
