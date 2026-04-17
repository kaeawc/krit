package rules

import (
	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// AndroidDataDependency describes which Android project data a rule needs.
type AndroidDataDependency uint32

const (
	AndroidDepNone     AndroidDataDependency = 0
	AndroidDepManifest AndroidDataDependency = 1 << iota
	AndroidDepLayout
	AndroidDepIcons
	AndroidDepGradle
	AndroidDepValuesStrings
	AndroidDepValuesDimensions
	AndroidDepValuesPlurals
	AndroidDepValuesArrays
	AndroidDepValuesExtraText
)

const AndroidDepValues = AndroidDepValuesStrings | AndroidDepValuesDimensions | AndroidDepValuesPlurals | AndroidDepValuesArrays | AndroidDepValuesExtraText
const AndroidDepResources = AndroidDepValues

// AndroidDependencyProvider can be implemented by rules that want to declare
// their Android project data needs explicitly.
type AndroidDependencyProvider interface {
	AndroidDependencies() AndroidDataDependency
}

// AndroidDependenciesOf returns the Android project data required by a rule.
// It prefers explicit metadata but falls back to the established rule
// interfaces and known icon-rule names.
func AndroidDependenciesOf(rule Rule) AndroidDataDependency {
	if provider, ok := rule.(AndroidDependencyProvider); ok {
		return provider.AndroidDependencies()
	}

	var deps AndroidDataDependency
	if _, ok := rule.(interface {
		CheckManifest(m *Manifest) []scanner.Finding
	}); ok {
		deps |= AndroidDepManifest
	}
	if _, ok := rule.(interface {
		CheckResources(idx *android.ResourceIndex) []scanner.Finding
	}); ok {
		deps |= AndroidDepValues | AndroidDepLayout
	}
	if _, ok := rule.(interface {
		CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding
	}); ok {
		deps |= AndroidDepGradle
	}
	if isIconRuleName(rule.Name()) {
		deps |= AndroidDepIcons
	}
	return deps
}

func isIconRuleName(name string) bool {
	switch name {
	case "IconDensities",
		"IconDipSize",
		"IconDuplicates",
		"GifUsage",
		"ConvertToWebp",
		"IconMissingDensityFolder",
		"IconExpectedSize",
		"IconNoDpi",
		"IconDuplicatesConfig":
		return true
	default:
		return false
	}
}
