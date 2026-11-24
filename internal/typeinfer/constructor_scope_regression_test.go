package typeinfer

import (
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestParallelIndexConstructorLocalResolutionDoesNotChange(t *testing.T) {
	model := parseTestFile(t, `package example.model
class Analytics(val type: String? = null) { fun copy(type: String?) = Analytics(type) }
`)
	file := parseTestFile(t, `package example
import example.model.Analytics
sealed class Key { class Conv(val analytics: Analytics?) : Key(); object Other : Key() }
fun Intent.build(): Analytics { val analytics = Analytics(type = "x"); return analytics }
fun Key.resolve(): String? = when (this) { is Key.Conv -> analytics?.type; else -> null }
`)
	resolver := NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{model, file}, 1)
	idx := flatFirstOfTypeWithText(file, "navigation_expression", "analytics?.type")
	if idx == 0 {
		t.Fatal("missing nullable member access")
	}
	receiver := file.FlatNamedChild(idx, 0)
	got := resolver.ResolveFlatNode(receiver, file)
	// Inside `fun Key.resolve()`, `analytics` is a member of the (smart-cast) receiver, so the lookup stops
	// at the extension receiver with an unknown type and never reaches build()'s `val analytics` local.
	want := UnknownType()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("constructor local resolution changed from main: got %+v, want %+v", got, want)
	}
}
