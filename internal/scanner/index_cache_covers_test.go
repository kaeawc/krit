package scanner

import "testing"

func TestCrossFileCacheMeta_CoversSourcePaths(t *testing.T) {
	meta := CrossFileCacheMeta{Entries: []fingerprintEntry{
		{Path: "A.kt", Hash: "1"},
		{Path: "B.java", Hash: "2"},
		{Path: "res/layout/main.xml", Hash: "3"},
	}}
	cases := []struct {
		name  string
		paths []string
		want  bool
	}{
		{"same set", []string{"B.java", "A.kt"}, true},
		{"deleted file", []string{"A.kt"}, false},
		{"added file", []string{"A.kt", "B.java", "C.kt"}, false},
		{"renamed file", []string{"A.kt", "C.java"}, false},
	}
	if (CrossFileCacheMeta{}).CoversSourcePaths([]string{"A.kt"}) {
		t.Error("old meta with empty Entries must be a miss")
	}
	for _, tc := range cases {
		if got := meta.CoversSourcePaths(tc.paths); got != tc.want {
			t.Errorf("%s: CoversSourcePaths=%t, want %t", tc.name, got, tc.want)
		}
	}
}
