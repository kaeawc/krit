package typeinfer

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// Regression for #1673: primary-constructor parameters are in scope in
// property initializers, delegates and init blocks, where they shadow
// same-named members; accessors and member functions see the member.
func TestPrimaryConstructorParamScopedToInitializers(t *testing.T) {
	src := `package demo

class Model(id: Long?, name: String?) {
    val a = id /*init*/
    val b by lazy { id /*delegate*/ }
    val id: Long
        get() = 0L + id.hashCode() /*getter*/

    init {
        println(id /*initBlock*/)
        val name = 1
        println(name /*shadowLocal*/)
    }

    fun read() = id /*member*/
    fun other(id: Long?) = id /*otherParam*/
}
`
	file := parseTestFile(t, src)
	resolver := NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)

	for _, test := range []struct {
		marker, name, wantName string
		wantNullable           bool
	}{
		{"id /*init*/", "id", "Long", true},
		{"id /*delegate*/", "id", "Long", true},
		{"id.hashCode() /*getter*/", "id", "Long", false},
		{"id /*initBlock*/", "id", "Long", true},
		{"name /*shadowLocal*/", "name", "Int", false},
		{"id /*member*/", "id", "Long", false},
		{"id /*otherParam*/", "id", "Long", true},
	} {
		t.Run(test.marker, func(t *testing.T) {
			offset := strings.Index(src, test.marker)
			if offset < 0 {
				t.Fatalf("marker %q not in source", test.marker)
			}
			got := resolver.resolveByNameAtOffset(test.name, uint32(offset), file)
			if got == nil {
				t.Fatalf("%s at %q did not resolve", test.name, test.marker)
			}
			if got.Name != test.wantName || got.IsNullable() != test.wantNullable {
				t.Fatalf("%s at %q = %s (nullable=%t), want %s (nullable=%t)", test.name, test.marker, got.Name, got.IsNullable(), test.wantName, test.wantNullable)
			}
		})
	}
}
