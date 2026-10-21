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

func TestCrossFileCacheMeta_CoversXMLFiles(t *testing.T) {
	meta := CrossFileCacheMeta{Entries: []fingerprintEntry{
		{Path: "A.kt", Hash: "1"},
		{Path: "res/layout/a.xml", Hash: "2"},
		{Path: "res/layout/b.xml", Hash: "3"},
	}}
	a := &XMLCacheFile{Path: "res/layout/a.xml", Hash: "2"}
	b := &XMLCacheFile{Path: "res/layout/b.xml", Hash: "3"}
	cases := []struct {
		name string
		xml  []*XMLCacheFile
		want bool
	}{
		{"same set", []*XMLCacheFile{b, a}, true},
		{"edited content", []*XMLCacheFile{a, {Path: b.Path, Hash: "4"}}, false},
		{"deleted file", []*XMLCacheFile{a}, false},
		{"added file", []*XMLCacheFile{a, b, {Path: "res/layout/c.xml", Hash: "5"}}, false},
		{"renamed file", []*XMLCacheFile{a, {Path: "res/layout/c.xml", Hash: "3"}}, false},
	}
	for _, tc := range cases {
		if got := meta.CoversXMLFiles(tc.xml); got != tc.want {
			t.Errorf("%s: CoversXMLFiles=%t, want %t", tc.name, got, tc.want)
		}
	}
	noXML := CrossFileCacheMeta{Entries: []fingerprintEntry{{Path: "A.kt", Hash: "1"}}}
	if !noXML.CoversXMLFiles(nil) {
		t.Error("no XML on either side must be covered")
	}
	if noXML.CoversXMLFiles([]*XMLCacheFile{a}) {
		t.Error("an XML file added to an XML-free index must be a miss")
	}
}
