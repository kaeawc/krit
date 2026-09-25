package scanner

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// krit-fir checkers cannot call IsTestFile, so a checker whose Go rule skips
// test files carries a copy of defaultTestPaths as `testPathMarkers =
// listOf(...)`. This keeps every such copy equal to the Go list, so a change
// here fails until the copies follow.
func TestFirCheckerTestPathMarkersMatchDefaultTestPaths(t *testing.T) {
	root := filepath.Join("..", "..", "tools", "krit-fir", "src", "main", "kotlin")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("krit-fir sources not found: %v", err)
	}
	block := regexp.MustCompile(`(?s)testPathMarkers\s*=\s*listOf\((.*?)\n\s*\)`)
	literal := regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)
	want := defaultTestPathSlice()

	copies := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".kt") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range block.FindAllSubmatch(src, -1) {
			copies++
			var got []string
			for _, lit := range literal.FindAllSubmatch(m[1], -1) {
				got = append(got, string(lit[1]))
			}
			if !slices.Equal(got, want) {
				t.Errorf("%s: testPathMarkers = %q, want scanner.defaultTestPaths %q", path, got, want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if copies == 0 {
		t.Fatal("no krit-fir testPathMarkers copy found; update or delete this test")
	}
}
