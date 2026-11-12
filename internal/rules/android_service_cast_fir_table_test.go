package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The FIR twin keeps its own copy of serviceCastMap; this guards the two against drift.
func TestServiceCastMapMatchesFIRTwin(t *testing.T) {
	path := filepath.Join("..", "..", "tools", "krit-fir", "src", "main", "kotlin",
		"dev", "jasonpearson", "krit", "fir", "checkers", "androidlint", "ServiceCast.kt")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("FIR sources not available: %v", err)
	}
	fir := map[string]string{}
	for _, m := range regexp.MustCompile(`"([A-Z0-9_]+_SERVICE)" to "(\w+)"`).FindAllStringSubmatch(string(src), -1) {
		fir[m[1]] = m[2]
	}
	if len(fir) != len(serviceCastMap) {
		t.Errorf("FIR table has %d entries, Go has %d", len(fir), len(serviceCastMap))
	}
	for name, want := range serviceCastMap {
		if got := fir[name]; got != want {
			t.Errorf("%s: FIR has %q, Go has %q", name, got, want)
		}
	}
}
