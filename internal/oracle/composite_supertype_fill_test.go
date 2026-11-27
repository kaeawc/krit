package oracle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func sourceResolverFor(t testing.TB, codes ...string) typeinfer.TypeResolver {
	t.Helper()
	dir := t.TempDir()
	files := make([]*scanner.File, 0, len(codes))
	for i, code := range codes {
		path := filepath.Join(dir, fmt.Sprintf("Src%d.kt", i))
		if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseKotlinFileCached(context.Background(), path, nil)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	r := typeinfer.NewResolver()
	r.IndexFilesParallel(files, 1)
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

// The source index keys a nested class by package and simple name, last
// declaration winning, so only the enclosing class's FQN proves which class
// the oracle names. Each case asks for fqn with one unresolved supertype and
// wants the source supertypes of exactly that class, or none (#1928).
func TestCompositeClassHierarchyNestedFallbackMatchesEnclosingClass(t *testing.T) {
	for _, tc := range []struct {
		name  string
		codes []string
		fqn   string
		want  []string
	}{
		{"first of two same-named nested classes",
			[]string{"package com.example\n\nclass A {\n    open class Inner : AppCompatActivity()\n}\n\nclass B {\n    open class Inner : Fragment()\n}\n"},
			"com.example.A.Inner", []string{"AppCompatActivity"}},
		{"second of two same-named nested classes",
			[]string{"package com.example\n\nclass A {\n    open class Inner : AppCompatActivity()\n}\n\nclass B {\n    open class Inner : Fragment()\n}\n"},
			"com.example.B.Inner", []string{"Fragment"}},
		{"nested class beside a top-level shadow",
			[]string{"package com.example\n\nclass Outer {\n    open class Inner : AppCompatActivity()\n}\n\nopen class Inner : Fragment()\n"},
			"com.example.Outer.Inner", []string{"AppCompatActivity"}},
		{"top-level class beside a nested shadow",
			[]string{"package com.example\n\nopen class Inner : Fragment()\n\nclass Outer {\n    open class Inner : AppCompatActivity()\n}\n"},
			"com.example.Inner", []string{"Fragment"}},
		{"nested class is not a top-level class of the same name",
			[]string{"package a.b\n\nclass X {\n    open class Inner : Fragment()\n}\n"},
			"a.b.Inner", nil},
		{"top-level class only",
			[]string{"package com.example\n\nopen class Inner : Fragment()\n"},
			"com.example.Outer.Inner", nil},
		{"nested class under a shorter package",
			[]string{"package com\n\nclass X {\n    open class Inner : Fragment()\n}\n"},
			"com.example.Outer.Inner", nil},
		{"nested class of another outer in a shorter package",
			[]string{"package com\n\nclass Outer {\n    open class Inner : Fragment()\n}\n"},
			"com.example.Outer.Inner", nil},
		{"local class",
			[]string{"package com.example\n\nfun f() {\n    open class Inner : Fragment()\n}\n"},
			"com.example.Outer.Inner", nil},
		{"local class inside the outer class",
			[]string{"package com.example\n\nclass Outer {\n    fun f() {\n        open class Inner : Fragment()\n    }\n}\n"},
			"com.example.Outer.Inner", nil},
		{"two levels deep",
			[]string{"package com.example\n\nclass A {\n    class B {\n        open class Inner : AppCompatActivity()\n    }\n}\n"},
			"com.example.A.B.Inner", []string{"AppCompatActivity"}},
		{"inside an object",
			[]string{"package com.example\n\nobject Holder {\n    open class Inner : AppCompatActivity()\n}\n"},
			"com.example.Holder.Inner", []string{"AppCompatActivity"}},
		{"inside a companion object",
			[]string{"package com.example\n\nclass Outer {\n    companion object {\n        open class Inner : AppCompatActivity()\n    }\n}\n"},
			"com.example.Outer.Companion.Inner", nil}, // not indexed from source yet (#1944)
		{"inside a named companion object",
			[]string{"package com.example\n\nclass Outer {\n    companion object Factory {\n        open class Inner : AppCompatActivity()\n    }\n}\n"},
			"com.example.Outer.Factory.Inner", nil}, // not indexed from source yet (#1944)
		{"inside an enum class",
			[]string{"package com.example\n\nenum class E {\n    ONE;\n\n    open class Inner : AppCompatActivity()\n}\n"},
			"com.example.E.Inner", nil}, // not indexed from source yet (#1944)
		{"default package",
			[]string{"class Outer {\n    open class Inner : AppCompatActivity()\n}\n"},
			"Outer.Inner", []string{"AppCompatActivity"}},
		{"same outer name in another package",
			[]string{"package com.other\n\nclass Outer {\n    open class Inner : Fragment()\n}\n",
				"package com.example\n\nclass Outer {\n    open class Inner : AppCompatActivity()\n}\n"},
			"com.example.Outer.Inner", []string{"AppCompatActivity"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := sourceResolverFor(t, tc.codes...)
			fake := NewFakeOracle()
			simple := tc.fqn[strings.LastIndexByte(tc.fqn, '.')+1:]
			fake.Classes[tc.fqn] = &typeinfer.ClassInfo{
				Name: simple, FQN: tc.fqn,
				Supertypes: []string{"??? (Unresolved qualified name: Base)"},
			}
			got := NewCompositeResolver(fake, src).ClassHierarchy(tc.fqn)
			if !equalStringSlices(got.Supertypes, tc.want) {
				t.Fatalf("Supertypes = %q, want %q", got.Supertypes, tc.want)
			}
		})
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

// An unresolved supertype on a class the source index does not hold is the
// common miss: it must not cost a framework-table scan per package prefix.
func BenchmarkCompositeClassHierarchySourceMiss(b *testing.B) {
	src := sourceResolverFor(b, "package com.example\n\nopen class Unrelated : Fragment()\n")
	fake := NewFakeOracle()
	const fqn = "com.example.feature.screens.Outer.Inner"
	fake.Classes[fqn] = &typeinfer.ClassInfo{
		Name: "Inner", FQN: fqn,
		Supertypes: []string{"??? (Unresolved qualified name: Base)"},
	}
	c := NewCompositeResolver(fake, src)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.ClassHierarchy(fqn)
	}
}
