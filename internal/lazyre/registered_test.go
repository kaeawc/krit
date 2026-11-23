package lazyre_test

import (
	"testing"

	_ "github.com/kaeawc/krit/internal/android"
	_ "github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/lazyre"
	_ "github.com/kaeawc/krit/internal/librarymodel"
	_ "github.com/kaeawc/krit/internal/module"
	_ "github.com/kaeawc/krit/internal/rules"
)

// TestDeclaredPatternsCompile replaces the startup check regexp.MustCompile
// used to give: every lazily compiled package-level pattern must compile.
func TestDeclaredPatternsCompile(t *testing.T) {
	if n := lazyre.Count(); n < 250 {
		t.Fatalf("expected the package-level patterns to be registered, got %d", n)
	}
	if err := lazyre.CompileAll(); err != nil {
		t.Fatal(err)
	}
}
