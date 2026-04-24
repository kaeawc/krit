// Descriptor metadata for internal/rules/release_engineering.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *AllProjectsBlockRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "AllProjectsBlock",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects deprecated allprojects {} blocks in Gradle build scripts.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *BuildConfigDebugInLibraryRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "BuildConfigDebugInLibrary",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects BuildConfig.DEBUG references inside Android library modules where the value is always false in release.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *BuildConfigDebugInvertedRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "BuildConfigDebugInverted",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects negated BuildConfig.DEBUG guards wrapping logging calls that likely invert a debug-only check.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CommentedOutCodeBlockRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "CommentedOutCodeBlock",
		RuleSet:       "release-engineering",
		Severity:      "info",
		Description:   "Detects consecutive lines of commented-out Kotlin code that should be deleted or restored.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CommentedOutImportRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "CommentedOutImport",
		RuleSet:       "release-engineering",
		Severity:      "info",
		Description:   "Detects commented-out import statements that are either dead code or incomplete refactors.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.9,
	}
}

func (r *ConventionPluginDeadCodeRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ConventionPluginDeadCode",
		RuleSet:       "release-engineering",
		Severity:      "info",
		Description:   "Detects convention plugins under build-logic or buildSrc that are never applied by any module.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DebugToastInProductionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "DebugToastInProduction",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects Toast.makeText calls whose message starts with debug-related prefixes in production code.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *GradleBuildContainsTodoRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "GradleBuildContainsTodo",
		RuleSet:       "release-engineering",
		Severity:      "info",
		Description:   "Detects TODO comments in build.gradle(.kts) files that may block release readiness.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *HardcodedEnvironmentNameRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "HardcodedEnvironmentName",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects hardcoded environment names like 'dev', 'staging', or 'prod' passed to config APIs.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *HardcodedLocalhostUrlRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "HardcodedLocalhostUrl",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects hardcoded localhost or 10.0.2.2 URLs in non-test production source files.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *HardcodedLogTagRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "HardcodedLogTag",
		RuleSet:       "release-engineering",
		Severity:      "info",
		Description:   "Detects Log tag string literals matching the enclosing class name instead of using a companion TAG constant.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.8,
	}
}

func (r *MergeConflictMarkerLeftoverRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MergeConflictMarkerLeftover",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects unresolved merge conflict markers (<<<, ===, >>>) left in source files.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.99,
	}
}

func (r *NonAsciiIdentifierRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "NonAsciiIdentifier",
		RuleSet:       "release-engineering",
		Severity:      "info",
		Description:   "Detects class, function, or property names containing non-ASCII characters.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *OpenForTestingCallerInNonTestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "OpenForTestingCallerInNonTest",
		RuleSet:       "release-engineering",
		Severity:      "info",
		Description:   "Detects subclassing of @OpenForTesting types outside test source sets.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *PrintStackTraceInProductionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "PrintStackTraceInProduction",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects printStackTrace() calls in code that has a logging framework available.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *PrintlnInProductionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "PrintlnInProduction",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects println or print calls in production code that should use a logging framework.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *TestFixtureAccessedFromProductionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "TestFixtureAccessedFromProduction",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects usage of types declared under src/testFixtures/ from production source files.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.8,
	}
}

func (r *TestOnlyImportInProductionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "TestOnlyImportInProduction",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects test framework imports (JUnit, Mockito, MockK, etc.) in non-test source files.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *TimberTreeNotPlantedRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "TimberTreeNotPlanted",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects Timber logging usage without any Timber.plant() call in the project.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *VisibleForTestingCallerInNonTestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "VisibleForTestingCallerInNonTest",
		RuleSet:       "release-engineering",
		Severity:      "warning",
		Description:   "Detects calls to @VisibleForTesting-annotated functions from non-test source files.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.8,
	}
}
