package oracle

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeKt(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func demotedBy(t *testing.T, newFile string, hitFiles map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	var hits []*CacheEntry
	for name, body := range hitFiles {
		hits = append(hits, &CacheEntry{FilePath: writeKt(t, dir, name, body)})
	}
	miss := writeKt(t, dir, "New.kt", newFile)
	kept, misses := demoteNameCollidingHits(InvocationOptions{Backend: BackendKAA}, hits, []string{miss}, nil, false)
	var demoted []string
	for _, m := range misses {
		if m != miss {
			demoted = append(demoted, filepath.Base(m))
		}
	}
	sort.Strings(demoted)
	if len(kept)+len(demoted) != len(hits) {
		t.Fatalf("hits not conserved: kept=%d demoted=%d total=%d", len(kept), len(demoted), len(hits))
	}
	return demoted
}

func TestDemoteNameCollidingHits(t *testing.T) {
	cases := []struct {
		name string
		new  string
		hits map[string]string
		want []string
	}{
		{
			name: "same package overload demotes caller",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nfun usePick() = pick(1)!!\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "unrelated new file demotes nothing",
			new:  "package p\nfun somethingElse(): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nfun usePick() = pick(1)!!\n"},
		},
		{
			name: "other package without import is not demoted",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package q\nfun usePick() = pick(1)\n"},
		},
		{
			name: "explicit import demotes",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package q\nimport p.pick\nfun usePick() = pick(1)\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "wildcard import demotes",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package q\nimport p.*\nfun usePick() = pick(1)\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "qualified use demotes",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package q\nfun usePick() = p.pick(1)\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "longer identifier is not a mention",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nfun pickAll() = picker()\n"},
		},
		{
			name: "private declaration is invisible",
			new:  "package p\nprivate fun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nfun usePick() = pick(1)\n"},
		},
		{
			name: "member declaration is not top level",
			new:  "package p\nclass Box { fun pick(x: Int): Int = 1 }\n",
			hits: map[string]string{"Use.kt": "package p\nfun usePick() = pick(1)\n"},
		},
		{
			name: "extension and property and type",
			new:  "package p\nfun String.pick(): Int = 1\nval limit = 3\nclass Widget\n",
			hits: map[string]string{
				"A.kt": "package p\nfun a() = \"x\".pick()\n",
				"B.kt": "package p\nfun b() = limit\n",
				"C.kt": "package p\nfun c(w: Widget) = w\n",
				"D.kt": "package p\nfun d() = 1\n",
			},
			want: []string{"A.kt", "B.kt", "C.kt"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := demotedBy(t, tc.new, tc.hits)
			if len(got) != len(tc.want) {
				t.Fatalf("demoted %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("demoted %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestDemoteNameCollidingHitsSkipsWholeCompilationBackend(t *testing.T) {
	dir := t.TempDir()
	hit := &CacheEntry{FilePath: writeKt(t, dir, "Use.kt", "package p\nfun u() = pick(1)\n")}
	miss := writeKt(t, dir, "New.kt", "package p\nfun pick(x: Int): Int = 1\n")
	hits, misses := demoteNameCollidingHits(InvocationOptions{Backend: BackendFIR}, []*CacheEntry{hit}, []string{miss}, nil, false)
	if len(hits) != 1 || len(misses) != 1 {
		t.Fatalf("FIR hits=%d misses=%d, want untouched", len(hits), len(misses))
	}
}
