package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestOptInMarkerWellKnownTableMatchesFIR(t *testing.T) {
	path := filepath.Join("..", "..", "tools", "krit-fir", "src", "main", "kotlin",
		"dev", "jasonpearson", "krit", "fir", "checkers", "licensing", "WellKnownOptInMarkers.kt")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	markers := map[string]bool{}
	for _, match := range regexp.MustCompile(`"([A-Za-z0-9]+)"`).FindAllStringSubmatch(string(source), -1) {
		if markers[match[1]] {
			t.Errorf("duplicate FIR marker %s", match[1])
		}
		markers[match[1]] = true
	}
	if len(markers) != len(wellKnownOptInMarkers) {
		t.Errorf("FIR table has %d entries, Go has %d", len(markers), len(wellKnownOptInMarkers))
	}
	for marker := range wellKnownOptInMarkers {
		if !markers[marker] {
			t.Errorf("FIR table is missing %s", marker)
		}
	}
}
