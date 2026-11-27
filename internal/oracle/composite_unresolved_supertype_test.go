package oracle

import (
	"testing"

	"github.com/kaeawc/krit/internal/typeinfer"
)

// hierarchyOnlyFallback answers ClassHierarchy from a map, by FQN or, for a
// name missing there, by simple name, as a source resolver given a short
// name may.
type hierarchyOnlyFallback struct {
	typeinfer.TypeResolver
	classes  map[string]*typeinfer.ClassInfo
	bySimple map[string]*typeinfer.ClassInfo
}

func (f hierarchyOnlyFallback) ClassHierarchy(name string) *typeinfer.ClassInfo {
	if c, ok := f.classes[name]; ok {
		return c
	}
	return f.bySimple[simpleSupertypeName(name)]
}

// krit-fir renders a supertype outside its classpath as an error type, which
// cut CustomSplashScreen's walk from SplashActivity through BaseActivity to
// AppCompatActivity. The source resolver's direct supertypes fill the gap;
// resolved oracle supertypes stay (#1869).
func TestCompositeClassHierarchyFillsUnresolvedSupertypesFromSource(t *testing.T) {
	fake := NewFakeOracle()
	// The oracle's ClassInfo carries Supertypes only (convertClassInfo).
	fake.Classes["com.example.BaseActivity"] = &typeinfer.ClassInfo{
		Name: "BaseActivity", FQN: "com.example.BaseActivity",
		Supertypes: []string{"??? (Unresolved qualified name: AppCompatActivity)", "com.example.Tracked"},
	}
	fake.Classes["com.example.Plain"] = &typeinfer.ClassInfo{Name: "Plain", FQN: "com.example.Plain", Supertypes: []string{"kotlin.Any"}}
	source := hierarchyOnlyFallback{classes: map[string]*typeinfer.ClassInfo{
		"com.example.BaseActivity": {
			Name: "BaseActivity", FQN: "com.example.BaseActivity",
			// Supertypes also holds the generic argument of Callback<Throwable>.
			Supertypes:       []string{"AppCompatActivity", "Tracked", "Callback", "Throwable"},
			DirectSupertypes: []string{"AppCompatActivity", "Tracked", "Callback"},
		},
		"com.example.Plain": {Name: "Plain", FQN: "com.example.Plain", Supertypes: []string{"Other"}},
	}}
	c := NewCompositeResolver(fake, source)

	got := c.ClassHierarchy("com.example.BaseActivity")
	// Tracked is the resolved com.example.Tracked written short; Throwable is
	// a type argument, not a supertype.
	if want := []string{"com.example.Tracked", "AppCompatActivity", "Callback"}; !equalStringSlices(got.Supertypes, want) {
		t.Errorf("Supertypes = %q, want %q", got.Supertypes, want)
	}
	if fake.Classes["com.example.BaseActivity"].Supertypes[0] != "??? (Unresolved qualified name: AppCompatActivity)" {
		t.Error("the oracle's shared entry was modified")
	}
	// A fully resolved oracle entry is returned as is.
	if got := c.ClassHierarchy("com.example.Plain"); !equalStringSlices(got.Supertypes, []string{"kotlin.Any"}) {
		t.Errorf("resolved entry: Supertypes = %q, want [kotlin.Any]", got.Supertypes)
	}
}

// The source class must be the oracle's class: com.a.BaseActivity must not
// take the ancestry of a same-named com.b.BaseActivity the source resolver
// finds by simple name.
func TestCompositeClassHierarchyIgnoresSourceLookalikeClass(t *testing.T) {
	fake := NewFakeOracle()
	fake.Classes["com.a.BaseActivity"] = &typeinfer.ClassInfo{
		Name: "BaseActivity", FQN: "com.a.BaseActivity",
		Supertypes: []string{"??? (Unresolved qualified name: AppCompatActivity)"},
	}
	source := hierarchyOnlyFallback{bySimple: map[string]*typeinfer.ClassInfo{
		"BaseActivity": {Name: "BaseActivity", FQN: "com.b.BaseActivity", DirectSupertypes: []string{"Fragment"}},
	}}
	got := NewCompositeResolver(fake, source).ClassHierarchy("com.a.BaseActivity")
	if len(got.Supertypes) != 0 {
		t.Fatalf("Supertypes = %q, want none: com.b.BaseActivity's ancestry was borrowed", got.Supertypes)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
