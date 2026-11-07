package pipeline

import (
	"fmt"
	"strings"
	"testing"
)

func TestLogUnmappedOracleModules_ListsPathsAndCapsTheList(t *testing.T) {
	var lines []string
	logf := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }

	logUnmappedOracleModules(nil, logf)
	if len(lines) != 0 {
		t.Fatalf("no unmapped files must log nothing, got %v", lines)
	}

	var paths []string
	for i := 0; i < maxLoggedUnmappedFiles+3; i++ {
		paths = append(paths, fmt.Sprintf("/repo/sample%d/A.kt", i))
	}
	logUnmappedOracleModules(paths, logf)
	out := strings.Join(lines, "")
	for _, want := range []string{"missing from settings", "unmapped: /repo/sample0/A.kt", "and 3 more"} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, fmt.Sprintf("sample%d/A.kt", maxLoggedUnmappedFiles)) {
		t.Errorf("paths past the cap must not be listed:\n%s", out)
	}
}
