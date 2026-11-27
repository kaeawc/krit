package oracle

import (
	"testing"

	"github.com/kaeawc/krit/internal/typeinfer"
)

type hierarchyOnlyFallback struct {
	typeinfer.TypeResolver
	classes map[string]*typeinfer.ClassInfo
}

func (f hierarchyOnlyFallback) ClassHierarchy(name string) *typeinfer.ClassInfo {
	return f.classes[name]
}

// krit-fir renders a supertype outside its classpath as an error type, which
// cut CustomSplashScreen's walk from SplashActivity through BaseActivity to
// AppCompatActivity. The source resolver's supertypes fill the gap; resolved
// oracle supertypes stay (#1869).
func TestCompositeClassHierarchyFillsUnresolvedSupertypesFromSource(t *testing.T) {
	fake := NewFakeOracle()
	fake.Classes["com.example.BaseActivity"] = &typeinfer.ClassInfo{
		Name: "BaseActivity", FQN: "com.example.BaseActivity",
		Supertypes:       []string{"??? (Unresolved qualified name: AppCompatActivity)", "com.example.Tracked"},
		DirectSupertypes: []string{"??? (Unresolved qualified name: AppCompatActivity)"},
	}
	fake.Classes["com.example.Plain"] = &typeinfer.ClassInfo{Name: "Plain", FQN: "com.example.Plain", Supertypes: []string{"kotlin.Any"}}
	source := hierarchyOnlyFallback{classes: map[string]*typeinfer.ClassInfo{
		"com.example.BaseActivity": {Name: "BaseActivity", FQN: "com.example.BaseActivity", Supertypes: []string{"AppCompatActivity"}, DirectSupertypes: []string{"AppCompatActivity"}},
		"com.example.Plain":        {Name: "Plain", FQN: "com.example.Plain", Supertypes: []string{"Other"}},
	}}
	c := NewCompositeResolver(fake, source)

	got := c.ClassHierarchy("com.example.BaseActivity")
	if want := []string{"com.example.Tracked", "AppCompatActivity"}; !equalStringSlices(got.Supertypes, want) {
		t.Errorf("Supertypes = %q, want %q", got.Supertypes, want)
	}
	if want := []string{"AppCompatActivity"}; !equalStringSlices(got.DirectSupertypes, want) {
		t.Errorf("DirectSupertypes = %q, want %q", got.DirectSupertypes, want)
	}
	if fake.Classes["com.example.BaseActivity"].Supertypes[0] == "AppCompatActivity" {
		t.Error("the oracle's shared entry was modified")
	}
	// A fully resolved oracle entry is returned as is.
	if got := c.ClassHierarchy("com.example.Plain"); !equalStringSlices(got.Supertypes, []string{"kotlin.Any"}) {
		t.Errorf("resolved entry: Supertypes = %q, want [kotlin.Any]", got.Supertypes)
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
