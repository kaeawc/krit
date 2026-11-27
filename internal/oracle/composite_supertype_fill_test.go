package oracle

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func sourceResolverFor(t *testing.T, code string) typeinfer.TypeResolver {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Src.kt")
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseKotlinFileCached(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := typeinfer.NewResolver()
	r.IndexFilesParallel([]*scanner.File{file}, 1)
	return r
}

// krit-fir names a nested class pkg.Outer.Inner; the source index names it
// pkg.Inner. The fill-in must still find it, or the #1869 fix skips every
// nested base activity (#1928).
func TestCompositeClassHierarchyFillsNestedClassFromSource(t *testing.T) {
	src := sourceResolverFor(t, "package com.example\n\nclass Outer {\n    open class Inner : AppCompatActivity()\n}\n")
	fake := NewFakeOracle()
	fake.Classes["com.example.Outer.Inner"] = &typeinfer.ClassInfo{
		Name: "Inner", FQN: "com.example.Outer.Inner",
		Supertypes: []string{"??? (Unresolved qualified name: AppCompatActivity)"},
	}
	got := NewCompositeResolver(fake, src).ClassHierarchy("com.example.Outer.Inner")
	if !equalStringSlices(got.Supertypes, []string{"AppCompatActivity"}) {
		t.Fatalf("Supertypes = %q, want [AppCompatActivity]", got.Supertypes)
	}
}

// A top-level class of the nested class's simple name, in another package,
// is not it.
func TestCompositeClassHierarchyNestedFallbackStaysInPackage(t *testing.T) {
	src := sourceResolverFor(t, "package com.other\n\nopen class Inner : Fragment()\n")
	fake := NewFakeOracle()
	fake.Classes["com.example.Outer.Inner"] = &typeinfer.ClassInfo{
		Name: "Inner", FQN: "com.example.Outer.Inner",
		Supertypes: []string{"??? (Unresolved qualified name: AppCompatActivity)"},
	}
	got := NewCompositeResolver(fake, src).ClassHierarchy("com.example.Outer.Inner")
	if len(got.Supertypes) != 0 {
		t.Fatalf("Supertypes = %q, want none: com.other.Inner's ancestry was borrowed", got.Supertypes)
	}
}

// A source name is the same type as a resolved oracle supertype only when it
// is that FQN or a dot-boundary suffix of it: Handler.Callback is not
// com.lib.SomeLib.Callback, and two unresolved names sharing a simple name
// both stay (#1928).
func TestMergeResolvedSupertypesKeepsDistinctSameSimpleNames(t *testing.T) {
	for _, tc := range []struct {
		name           string
		oracle, source []string
		want           []string
	}{
		{"qualified suffix of a resolved FQN is a duplicate",
			[]string{"??? (Unresolved qualified name: Handler.Callback)", "com.lib.SomeLib.Callback"},
			[]string{"Handler.Callback", "SomeLib.Callback"},
			[]string{"com.lib.SomeLib.Callback", "Handler.Callback"}},
		{"simple name written short is a duplicate",
			[]string{"??? (Unresolved qualified name: AppCompatActivity)", "com.example.Tracked"},
			[]string{"AppCompatActivity", "Tracked"},
			[]string{"com.example.Tracked", "AppCompatActivity"}},
		{"two unresolved names sharing a simple name both stay",
			[]string{"??? (Unresolved qualified name: Handler.Callback)", "??? (Unresolved qualified name: Other.Callback)"},
			[]string{"Handler.Callback", "Other.Callback"},
			[]string{"Handler.Callback", "Other.Callback"}},
		{"a suffix without a dot boundary is distinct",
			[]string{"??? (Unresolved qualified name: Callback)", "com.lib.MyCallback"},
			[]string{"Callback"},
			[]string{"com.lib.MyCallback", "Callback"}},
	} {
		if got := mergeResolvedSupertypes(tc.oracle, tc.source); !equalStringSlices(got, tc.want) {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// simpleNameLookup answers LookupClass by simple name too, as the real
// oracle's class index may, so a caller's short name finds the class.
type simpleNameLookup struct {
	*FakeOracle
	bySimple map[string]*typeinfer.ClassInfo
}

func (s simpleNameLookup) LookupClass(name string) *typeinfer.ClassInfo {
	if c := s.FakeOracle.LookupClass(name); c != nil {
		return c
	}
	return s.bySimple[name]
}

// Called with the short name, the fill-in must look the source class up by
// the oracle's FQN, not by the caller's name, which the source resolver could
// answer with a same-named class elsewhere.
func TestCompositeClassHierarchyLooksUpSourceByOracleFQN(t *testing.T) {
	info := &typeinfer.ClassInfo{
		Name: "BaseActivity", FQN: "com.a.BaseActivity",
		Supertypes: []string{"??? (Unresolved qualified name: AppCompatActivity)"},
	}
	oracle := simpleNameLookup{FakeOracle: NewFakeOracle(), bySimple: map[string]*typeinfer.ClassInfo{"BaseActivity": info}}
	oracle.Classes["com.a.BaseActivity"] = info
	source := hierarchyOnlyFallback{classes: map[string]*typeinfer.ClassInfo{
		"com.a.BaseActivity": {Name: "BaseActivity", FQN: "com.a.BaseActivity", DirectSupertypes: []string{"AppCompatActivity"}},
		"BaseActivity":       {Name: "BaseActivity", FQN: "com.b.BaseActivity", DirectSupertypes: []string{"Fragment"}},
	}}
	got := NewCompositeResolver(oracle, source).ClassHierarchy("BaseActivity")
	if !equalStringSlices(got.Supertypes, []string{"AppCompatActivity"}) {
		t.Fatalf("Supertypes = %q, want [AppCompatActivity] from com.a.BaseActivity", got.Supertypes)
	}
}
