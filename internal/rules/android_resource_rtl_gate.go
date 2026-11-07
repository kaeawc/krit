package rules

import (
	"path/filepath"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// rtlGatedRuleIDs are the rules whose reach depends on the RTL settings of the app that includes a module.
var rtlGatedRuleIDs = []string{
	"RtlHardcodedResource", "RtlSymmetryResource", "RtlSuperscriptResource", "RtlCompatResource", "RtlEnabledManifest",
}

// RtlGatedRuleIDs returns the IDs of the rules that need the pipeline to resolve main projects.
func RtlGatedRuleIDs() []string { return append([]string(nil), rtlGatedRuleIDs...) }

// rtlAPI is the first API level with native start/end and textAlignment support.
const rtlAPI = 17

// Lint's RtlDetector tags each incident with the conditions that must hold in the main project when the
// incident is reported. These are those tags (RtlDetector.applies).
const (
	// rtlTargetBelow17 needs the main project to target below API 17.
	rtlTargetBelow17 = 1
	// rtlEnabled needs the main project to have RTL on: target 17+ and an <application> without supportsRtl="false".
	rtlEnabled = 2
	// rtlMinBelow17 needs the main project to have a minSdk below 17.
	rtlMinBelow17 = 4
	// rtlMinAtLeast17 needs the main project to have a minSdk of 17 or more.
	rtlMinAtLeast17 = 8
	// rtlModuleModern needs the module to be modern: its minSdk, the including app's minSdk or the file's -v folder
	// is 17 or more. Lint reads a library's minSdk as at least its app's.
	rtlModuleModern = 16
)

// rtlMain is an api.RtlMainProject with its SDK levels resolved. A level the scan cannot find gets Lint's
// default (minSdk 1, targetSdk = minSdk) once the project's apps are known. Before that, a zero level is
// unknown and counts as modern.
type rtlMain struct {
	enabled bool
	minSdk  int
	target  int
}

// rtlResolveMains turns the main projects deciding a file into resolved ones, falling back to the project
// profile for unknown levels. Without project data it assumes one ordinary app with RTL on.
func rtlResolveMains(ctx *api.Context, facts *librarymodel.Facts, path string) []rtlMain {
	var in []api.RtlMainProject
	known := false
	if ctx.RtlMainsFor != nil {
		in, known = ctx.RtlMainsFor(path)
	}
	if !known {
		in = []api.RtlMainProject{{HasApplication: true}}
	}
	lenient := !known
	out := make([]rtlMain, 0, len(in))
	for _, m := range in {
		min, target := m.MinSdk, m.TargetSdk
		if facts != nil {
			if min == 0 {
				min = facts.Profile.MinSdkVersion
			}
			if target == 0 {
				target = facts.Profile.TargetSdkVersion
			}
		}
		if !lenient && min == 0 {
			min = 1
		}
		// Gradle gives an unset targetSdk the minSdk.
		if target == 0 {
			target = min
		}
		out = append(out, rtlMain{
			enabled: m.HasApplication && !m.RtlOff && (target == 0 || target >= rtlAPI),
			minSdk:  min,
			target:  target,
		})
	}
	return out
}

// rtlModule is what the detector reads from the module that owns a layout file.
type rtlModule struct {
	minSdk, target int
	// modern is true when the module's minSdk or the folder's version is 17+, or when neither is known.
	modern bool
}

// rtlModuleCache reads the SDK levels of the module owning a layout file, once per resource root.
type rtlModuleCache struct {
	facts *librarymodel.Facts
	// lenient keeps unknown levels at 0 (modern) because the project's apps are unknown.
	lenient bool
	byRes   map[string]android.SDKLevels
}

func newRtlModuleCache(ctx *api.Context, facts *librarymodel.Facts) *rtlModuleCache {
	return &rtlModuleCache{facts: facts, lenient: ctx.RtlMainsFor == nil, byRes: map[string]android.SDKLevels{}}
}

// forLayout returns the module of a layout file.
func (c *rtlModuleCache) forLayout(path string) rtlModule {
	res := filepath.Dir(filepath.Dir(path))
	levels, ok := c.byRes[res]
	if !ok {
		levels = android.ResolveSDKLevels(path)
		if c.facts != nil {
			if levels.MinSdk == 0 {
				levels.MinSdk = c.facts.Profile.MinSdkVersion
			}
			if levels.TargetSdk == 0 {
				levels.TargetSdk = c.facts.Profile.TargetSdkVersion
			}
		}
		if !c.lenient && levels.MinSdk == 0 {
			levels.MinSdk = 1
		}
		c.byRes[res] = levels
	}
	return rtlModule{
		minSdk: levels.MinSdk,
		target: levels.TargetSdk,
		modern: levels.MinSdk == 0 || levels.MinSdk >= rtlAPI || iconFolderVersion(path) >= rtlAPI,
	}
}

// rtlApplies reports whether some main project passes the incident's tags, and returns the first one that does.
func rtlApplies(flags int, mains []rtlMain, module rtlModule) (rtlMain, bool) {
	for _, m := range mains {
		switch {
		case flags&rtlEnabled != 0 && !m.enabled:
		case flags&rtlTargetBelow17 != 0 && !(m.target > 0 && m.target < rtlAPI):
		case flags&rtlMinBelow17 != 0 && !(m.minSdk > 0 && m.minSdk < rtlAPI):
		case flags&rtlMinAtLeast17 != 0 && m.minSdk > 0 && m.minSdk < rtlAPI:
		case flags&rtlModuleModern != 0 && !module.modern && !(m.minSdk >= rtlAPI):
		default:
			return m, true
		}
	}
	return rtlMain{}, false
}
