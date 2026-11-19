package rules

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/typeinfer"
)

// A resolver index without DirectSupertypes keeps generic arguments on Supertypes.
func TestClassInfoDirectSupertypes_StripsArguments(t *testing.T) {
	info := &typeinfer.ClassInfo{Supertypes: []string{"ResourcesBase<RecyclerView.ViewHolder>", "Callback"}}
	if got := classInfoDirectSupertypes(info); strings.Join(got, ",") != "ResourcesBase,Callback" {
		t.Fatalf("expected generic arguments stripped, got %v", got)
	}
}
