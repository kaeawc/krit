package rules

// Android Manifest rules ported from AOSP ManifestDetector.
// These rules analyze AndroidManifest.xml rather than Kotlin source.
// They run once per project on the parsed manifest file.
//
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/
// Source: ManifestDetector.java, SecurityDetector.java

import (
	"github.com/kaeawc/krit/internal/manifest"
)

// ---------------------------------------------------------------------------
// Manifest data model — re-exported from internal/manifest so existing rule
// code and tests can continue to refer to rules.Manifest, rules.Manifest-
// Application, etc. The canonical definitions live in the leaf manifest
// package, which v2.Context can import without creating a cycle.
// ---------------------------------------------------------------------------

type (
	Manifest            = manifest.Manifest
	ManifestApplication = manifest.Application
	ManifestUsesFeature = manifest.UsesFeature
	ManifestMetaData    = manifest.MetaData
	ManifestComponent   = manifest.Component
	ManifestElement     = manifest.Element
)

// ManifestBase is an empty marker type embedded by manifest rule
// implementations. The rule registry source records AndroidDependencies()
// metadata on api.Rule.AndroidDeps.
type ManifestBase struct{}

// Confidence reports a tier-2 (medium) base confidence for manifest rules.
// Detection flags exported components, insecure flags, and overly broad
// permissions via manifest attribute checks; flavors or manifest merging can
// occasionally change the effective app manifest. Override per rule when the
// detection is purely structural.
func (ManifestBase) Confidence() float64 { return ConfidenceMedium }

func (ManifestBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepManifest
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// allComponents returns all components from the application element.
func allComponents(app *ManifestApplication) []ManifestComponent {
	return manifest.AllComponents(app)
}
