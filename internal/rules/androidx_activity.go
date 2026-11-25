package rules

// Ports of the source checks bundled in androidx.activity:activity 1.13.0 (activity-lint):
// ActivityResultFragmentVersionDetector and OnBackPressedDetector.

import (
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// InvalidFragmentVersionForActivityResultRule mirrors InvalidFragmentVersionForActivityResult: registerForActivityResult
// in a module whose androidx.fragment:fragment dependency is older than 1.3.0.
type InvalidFragmentVersionForActivityResultRule struct {
	FlatDispatchBase
	AndroidRule
}

// InvalidUseOfOnBackPressedRule mirrors InvalidUseOfOnBackPressed: an onBackPressed() call inside a method named
// handledOnBackPressed.
type InvalidUseOfOnBackPressedRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the dependency version comes from declared and catalog
// coordinates, not from the resolved dependency graph Lint reads.
func (r *InvalidFragmentVersionForActivityResultRule) Confidence() float64 {
	return api.ConfidenceMedium
}

// Confidence reports a high base confidence: Lint's check is a name match on the call and the enclosing method.
func (r *InvalidUseOfOnBackPressedRule) Confidence() float64 { return api.ConfidenceHigh }

func registerAndroidXActivityChecks() {
	callNodes := []string{"call_expression", "method_invocation"}
	version := &InvalidFragmentVersionForActivityResultRule{AndroidRule: lintAndroidRule("InvalidFragmentVersionForActivityResult",
		"Update to Fragment 1.3.0 to use ActivityResult APIs", ALCCorrectness, ALSFatal, 5)}
	registerLintRule(version.AndroidRule, version, version.Confidence(), version.check, callNodes, 0, nil)
	back := &InvalidUseOfOnBackPressedRule{AndroidRule: lintAndroidRule("InvalidUseOfOnBackPressed",
		"Do not call onBackPressed() within OnBackPressedDisptacher", ALCCorrectness, ALSWarning, 5)}
	registerLintRule(back.AndroidRule, back, back.Confidence(), back.check, callNodes, 0, nil)
}

func (r *InvalidFragmentVersionForActivityResultRule) check(ctx *api.Context) {
	call, ok := lintCallAt(ctx.File, ctx.Idx)
	// Lint matches the method name only.
	if !ok || call.Name != "registerForActivityResult" {
		return
	}
	if !activityModuleHasOldFragment(ctx) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Upgrade Fragment version to at least 1.3.0."))
}

// activityModuleHasOldFragment reports whether the build file of the source file's module declares
// androidx.fragment:fragment older than 1.3.0, directly or through the version catalog. Without a module build file it
// falls back to the project profile. A fragment version that arrives only transitively is not seen (#1120).
func activityModuleHasOldFragment(ctx *api.Context) bool {
	profile, found := activityModuleProfile(ctx.File.Path)
	if !found {
		profile = librarymodel.EnsureFacts(ctx.LibraryFacts).Profile
	}
	for _, dep := range profile.Dependencies {
		if dep.Group != "androidx.fragment" || dep.Name != "fragment" {
			continue
		}
		// Lint compares the text before the last '-' as a string, so 1.10.0 would count as older than 1.3.0; krit
		// compares the numbers. A pre-release of 1.3.0 counts as 1.3.0 in both.
		if v := parseGradleVersion(dep.Version); v.known && !v.atLeast(1, 3, 0) {
			return true
		}
	}
	return false
}

// activityModuleProfile reads the nearest build.gradle(.kts) above path, with its version catalog.
func activityModuleProfile(path string) (librarymodel.ProjectProfile, bool) {
	// The walk ends at the filesystem root, so a source file in a deep package still finds its module.
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		for _, name := range []string{"build.gradle.kts", "build.gradle"} {
			build := filepath.Join(dir, name)
			content, err := os.ReadFile(build)
			if err != nil {
				continue
			}
			catalog, _ := librarymodel.LoadVersionCatalogForGradlePath(build)
			return librarymodel.ProfileFromGradleContentWithCatalog(build, string(content), catalog), true
		}
		if parent := filepath.Dir(dir); parent == dir {
			break
		}
	}
	return librarymodel.ProjectProfile{}, false
}

// activityCoordinates lists artifacts that bring androidx.activity, whose AAR ships activity-lint.
var activityCoordinates = append([]librarymodel.Coordinate{
	{Group: "androidx.activity", Name: "activity"},
	{Group: "androidx.activity", Name: "activity-ktx"},
	{Group: "androidx.activity", Name: "activity-compose"},
}, fragmentCoordinates...)

func (r *InvalidUseOfOnBackPressedRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "onBackPressed" {
		return
	}
	// Lint only reports inside a method named handledOnBackPressed. OnBackPressedCallback's method is
	// handleOnBackPressed, so the shipped check matches only that exact (misspelled) name; krit keeps Lint's verdicts.
	if activityEnclosingMethodName(file, ctx.Idx) != "handledOnBackPressed" {
		return
	}
	if !librarymodel.EnsureFacts(ctx.LibraryFacts).Profile.HasAnyDependency(activityCoordinates...) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "",
		"Should not call onBackPressed inside of OnBackPressedCallback.handledOnBackPressed"))
}

// activityEnclosingMethodName names the nearest enclosing function or method, looking through lambdas as UAST's
// getParentOfType<UMethod> does, or returns "" when the call is not inside one.
func activityEnclosingMethodName(file *scanner.File, idx uint32) string {
	fn, ok := flatEnclosingAncestor(file, idx, "function_declaration", "method_declaration")
	if !ok {
		return ""
	}
	if file.FlatType(fn) == "method_declaration" {
		if id, ok := file.FlatFindChild(fn, "identifier"); ok {
			return file.FlatNodeText(id)
		}
		return ""
	}
	if id, ok := file.FlatFindChild(fn, "simple_identifier"); ok {
		return file.FlatNodeText(id)
	}
	return ""
}
