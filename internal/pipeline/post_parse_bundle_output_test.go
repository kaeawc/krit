package pipeline

import (
	"testing"
)

// TestCanUsePostParseBundleOutputShortcut_DefaultArgsAllow pins the
// happy path: a daemon analyze with the standard daemonState wiring
// (BundleOutput / StoreBundleOutput populated, JSON+Compact format,
// no Fix/DryRun, no baseline/diff/severity tweaks) clears every
// gate. Without this, the kotlin-corpus warm+ABI BundleOutput
// shortcut never fires and we burn ~100 ms re-formatting findings
// on every cycle.
func TestCanUsePostParseBundleOutputShortcut_DefaultArgsAllow(t *testing.T) {
	host := ProjectHostState{
		DaemonCaches: DaemonCaches{
			BundleOutput:      func(string) *CachedBundleOutput { return nil },
			StoreBundleOutput: func(string, *CachedBundleOutput) {},
		},
	}
	args := ProjectArgs{Format: "json", JSONCompact: true}

	if !canUsePostParseBundleOutputShortcut(args, host) {
		t.Errorf("default daemon-shaped args + host must allow the shortcut")
	}
}

// TestCanUsePostParseBundleOutputShortcut_FixForcesFullPath pins
// the correctness envelope: args.Fix needs FixupPhase to apply
// changes to the FindingColumns before serialization. The cached
// bytes pre-date Fixup, so taking the shortcut would silently drop
// fix application. Same shape as tryLoadFindingsBundleBeforeParse's
// gate.
func TestCanUsePostParseBundleOutputShortcut_FixForcesFullPath(t *testing.T) {
	host := ProjectHostState{
		DaemonCaches: DaemonCaches{
			BundleOutput:      func(string) *CachedBundleOutput { return nil },
			StoreBundleOutput: func(string, *CachedBundleOutput) {},
		},
	}
	for _, tc := range []struct {
		name string
		args ProjectArgs
	}{
		{"Fix", ProjectArgs{Format: "json", JSONCompact: true, Fix: true}},
		{"FixBinary", ProjectArgs{Format: "json", JSONCompact: true, FixBinary: true}},
		{"DryRun", ProjectArgs{Format: "json", JSONCompact: true, DryRun: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if canUsePostParseBundleOutputShortcut(tc.args, host) {
				t.Errorf("%s must force the full FixupPhase+OutputPhase route", tc.name)
			}
		})
	}
}

// TestCanUsePostParseBundleOutputShortcut_CustomRuleJarsForcesFullPath
// covers the custom-rule-jars case: those rules' outputs aren't
// captured in the cached BundleOutput bytes (the cache is keyed on
// the built-in rule set's fingerprint), so the shortcut would serve
// findings missing the custom-jar contributions.
func TestCanUsePostParseBundleOutputShortcut_CustomRuleJarsForcesFullPath(t *testing.T) {
	host := ProjectHostState{
		DaemonCaches: DaemonCaches{
			BundleOutput:      func(string) *CachedBundleOutput { return nil },
			StoreBundleOutput: func(string, *CachedBundleOutput) {},
		},
	}
	args := ProjectArgs{
		Format:         "json",
		JSONCompact:    true,
		CustomRuleJars: []string{"/path/to/custom.jar"},
	}

	if canUsePostParseBundleOutputShortcut(args, host) {
		t.Errorf("custom-rule-jars must force the full route")
	}
}

// TestCanUsePostParseBundleOutputShortcut_ResponseShapeGatesPassThrough
// pins the layered relationship with canUseBundleOutputCache: anything
// that ROAMS the response shape (baseline filter, format, etc.) must
// still reject the shortcut even when Fix gates are clear.
func TestCanUsePostParseBundleOutputShortcut_ResponseShapeGatesPassThrough(t *testing.T) {
	host := ProjectHostState{
		DaemonCaches: DaemonCaches{
			BundleOutput:      func(string) *CachedBundleOutput { return nil },
			StoreBundleOutput: func(string, *CachedBundleOutput) {},
		},
	}
	for _, tc := range []struct {
		name string
		args ProjectArgs
	}{
		{"baseline filter", ProjectArgs{Format: "json", JSONCompact: true, BaselinePath: "/baseline.xml"}},
		{"diff filter", ProjectArgs{Format: "json", JSONCompact: true, DiffRef: "main"}},
		{"warnings-as-errors", ProjectArgs{Format: "json", JSONCompact: true, WarningsAsErrors: true}},
		{"min-confidence", ProjectArgs{Format: "json", JSONCompact: true, MinConfidence: 0.5}},
		{"non-json format", ProjectArgs{Format: "sarif", JSONCompact: true}},
		{"json non-compact", ProjectArgs{Format: "json", JSONCompact: false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if canUsePostParseBundleOutputShortcut(tc.args, host) {
				t.Errorf("%s must reject the shortcut via canUseBundleOutputCache passthrough", tc.name)
			}
		})
	}
}

// TestCanUsePostParseBundleOutputShortcut_NoStoreReturnsFalse pins
// the CLI-path contract: when the host hasn't wired BundleOutput
// (in-process CLI run, tests that don't care), the shortcut never
// fires and the regular pipeline executes.
func TestCanUsePostParseBundleOutputShortcut_NoStoreReturnsFalse(t *testing.T) {
	host := ProjectHostState{} // no BundleOutput / StoreBundleOutput
	args := ProjectArgs{Format: "json", JSONCompact: true}

	if canUsePostParseBundleOutputShortcut(args, host) {
		t.Errorf("nil BundleOutput must reject the shortcut")
	}
}
