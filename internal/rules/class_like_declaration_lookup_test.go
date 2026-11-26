package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/filefacts"
)

func TestFlatFindSameFileClassLikeDeclaration(t *testing.T) {
	file := parseInlineForInternalTest(t, `package p

class Outer {
    class Inner
    object Holder
}
object Holder
class Second
`)
	firstNamed := func(name string) uint32 {
		for i := uint32(1); int(i) < file.FlatTree.Len(); i++ {
			switch file.FlatType(i) {
			case "class_declaration", "object_declaration":
				if extractIdentifierFlat(file, i) == name {
					return i
				}
			}
		}
		return 0
	}
	prev := fileFactsCache()
	t.Cleanup(func() { setSharedFileFacts(prev) })
	// Without a run cache the helper recomputes; with one it reads the per-file index. Both must agree.
	for _, cache := range []*filefacts.Cache{nil, filefacts.NewCache()} {
		setSharedFileFacts(cache)
		for _, name := range []string{"Outer", "Inner", "Holder", "Second"} {
			want := firstNamed(name)
			if want == 0 {
				t.Fatalf("fixture has no declaration %s", name)
			}
			for range 2 {
				if got := flatFindSameFileClassLikeDeclaration(file, name); got != want {
					t.Errorf("cache=%v: lookup(%s) = %d, want the first declaration in document order, %d",
						cache != nil, name, got, want)
				}
			}
		}
		if got := flatFindSameFileClassLikeDeclaration(file, "Missing"); got != 0 {
			t.Errorf("cache=%v: lookup(Missing) = %d, want 0", cache != nil, got)
		}
	}
}
