package fileignore

import (
	"path/filepath"
	"testing"
)

func TestTestDataPathUnder(t *testing.T) {
	tests := []struct {
		name       string
		root, path string
		want       bool
	}{
		{name: "ancestor testdata above root", root: "/tmp/testdata/proj", path: "/tmp/testdata/proj/src/main/kotlin/A.kt", want: false},
		{name: "ancestor test resources above root", root: "/w/src/test/resources/proj", path: "/w/src/test/resources/proj/src/main/A.kt", want: false},
		{name: "testdata below root", root: "/tmp/proj", path: "/tmp/proj/src/test/testdata/A.kt", want: true},
		{name: "test data below root", root: "/tmp/proj", path: "/tmp/proj/src/test/data/A.kt", want: true},
		{name: "test resources below root", root: "/tmp/proj", path: "/tmp/proj/src/test/resources/A.kt", want: true},
		{name: "testdata directly under root", root: "/tmp/proj", path: "/tmp/proj/testdata/A.kt", want: true},
		{name: "relative root", root: ".", path: "testdata/A.kt", want: true},
		{name: "relative root clean", root: "proj", path: "proj/src/A.kt", want: false},
		{name: "no root checks full path", root: "", path: "project/src/testData/A.kt", want: true},
		{name: "path outside root checks full path", root: "/tmp/other", path: "/tmp/proj/testdata/A.kt", want: true},
		{name: "testdata-like file name", root: "/tmp/proj", path: "/tmp/proj/src/testdata.kt", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TestDataPathUnder(filepath.FromSlash(tt.root), filepath.FromSlash(tt.path)); got != tt.want {
				t.Errorf("TestDataPathUnder(%q, %q) = %t, want %t", tt.root, tt.path, got, tt.want)
			}
		})
	}
}
