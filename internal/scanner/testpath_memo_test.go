package scanner

import "testing"

func TestIsTestFileMemoFollowsInitTestPaths(t *testing.T) {
	t.Cleanup(func() { InitTestPaths(nil, nil) })
	InitTestPaths(nil, nil)
	const path = "module/src/custom/Foo.kt"
	if IsTestFile(path) {
		t.Fatalf("%s should not be a test file by default", path)
	}
	if IsTestFile(path) {
		t.Fatalf("memoized %s should not be a test file", path)
	}
	InitTestPaths([]string{"/custom/"}, nil)
	if !IsTestFile(path) {
		t.Fatalf("%s should be a test file after configuring /custom/", path)
	}
}
