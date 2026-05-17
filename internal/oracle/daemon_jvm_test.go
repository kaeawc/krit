package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// containsFlag reports whether args contains an entry equal to value or
// whose key (the part before "=") equals value. Lets the regression
// tests assert both bare flags ("-Xshare:auto") and key=value pairs
// ("-XX:AOTCache=/path/...").
func containsFlag(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
		if idx := strings.Index(arg, "="); idx > 0 && arg[:idx] == value {
			return true
		}
	}
	return false
}

func writeTempJar(t *testing.T) string {
	t.Helper()
	jar := filepath.Join(t.TempDir(), "krit-types.jar")
	if err := os.WriteFile(jar, []byte("fake jar"), 0644); err != nil {
		t.Fatalf("write temp jar: %v", err)
	}
	return jar
}

// TestAppendStartupCacheArgs_NoMixedCDSAndAOT is the regression for the
// JDK 25+ daemon-spawn failure: combining `-Xshare:auto`/CDS flags with
// `-XX:AOTCache=` or `-XX:AOTMode=record` aborts VM init with
// "Option AOTConfiguration cannot be used at the same time with
// -Xshare:on, -Xshare:auto, ...". appendStartupCacheArgs must pick one
// strategy per launch, never both.
func TestAppendStartupCacheArgs_NoMixedCDSAndAOT(t *testing.T) {
	// Sandbox HOME so cache paths land under TempDir and do not pollute
	// the developer's ~/.krit/cache.
	t.Setenv("HOME", t.TempDir())
	jar := writeTempJar(t)

	args := appendStartupCacheArgs(nil, "java", jar, false)

	usesAOT := containsFlag(args, "-XX:AOTCache") ||
		containsFlag(args, "-XX:AOTMode") ||
		containsFlag(args, "-XX:AOTConfiguration")
	usesCDS := containsFlag(args, "-XX:SharedArchiveFile") ||
		containsFlag(args, "-XX:ArchiveClassesAtExit") ||
		containsFlag(args, "-Xshare:auto")

	if usesAOT && usesCDS {
		t.Fatalf("appendStartupCacheArgs combined Leyden AOT and AppCDS flags (JDK 25+ refuses this combo):\n%v", args)
	}
}

// TestAppendLeydenAOTArgs_ReportsAddition makes sure the
// (args, addedAOT) contract holds: appendLeydenAOTArgs must return
// addedAOT=true exactly when it appended at least one AOT flag, so the
// wrapper can decide whether to skip AppCDS.
func TestAppendLeydenAOTArgs_ReportsAddition(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := writeTempJar(t)

	args, addedAOT := appendLeydenAOTArgs(nil, "java", jar, false)
	added := len(args) > 0

	if added != addedAOT {
		t.Fatalf("appendLeydenAOTArgs returned addedAOT=%v but appended=%v (args=%v)",
			addedAOT, added, args)
	}
}
