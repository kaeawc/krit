package rules

// Android Manifest rules ported from AOSP ManifestDetector.
// These rules analyze AndroidManifest.xml rather than Kotlin source.
// They run once per project on the parsed manifest file.
//
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/
// Source: ManifestDetector.java, SecurityDetector.java

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// Manifest data model — used by manifest-rule implementations
// ---------------------------------------------------------------------------

// Manifest represents a parsed AndroidManifest.xml.
type Manifest struct {
	Path            string           // file path to AndroidManifest.xml
	Package         string           // package attribute on <manifest>
	MinSDK          int              // android:minSdkVersion (0 if absent)
	TargetSDK       int              // android:targetSdkVersion (0 if absent)
	VersionCode     string           // android:versionCode on <manifest> ("" if absent)
	VersionName     string           // android:versionName on <manifest> ("" if absent)
	UsesSdk         *ManifestElement // <uses-sdk> element, nil if missing
	Application     *ManifestApplication
	Elements        []ManifestElement     // all top-level children of <manifest>
	UsesPermissions []string              // <uses-permission android:name="..."> values
	Permissions     []string              // <permission android:name="..."> values
	UsesFeatures    []ManifestUsesFeature // <uses-feature> elements
	IsDebugManifest bool                  // true if this is a debug build-variant manifest
	HasNativeLibs   bool                  // true if the project contains .so native libraries
}

// ManifestApplication represents the <application> element.
type ManifestApplication struct {
	Line                  int
	AllowBackup           *bool  // nil = not set, true/false = explicit
	Debuggable            *bool  // nil = not set
	LocaleConfig          string // android:localeConfig value
	Icon                  string // android:icon value
	UsesCleartextTraffic  *bool  // nil = not set, true/false = explicit
	FullBackupContent     string // android:fullBackupContent value
	DataExtractionRules   string // android:dataExtractionRules value
	SupportsRtl           *bool  // nil = not set, true/false = explicit
	ExtractNativeLibs     *bool  // nil = not set, true/false = explicit
	NetworkSecurityConfig string // android:networkSecurityConfig value
	Activities            []ManifestComponent
	Services              []ManifestComponent
	Receivers             []ManifestComponent
	Providers             []ManifestComponent
}

// ManifestUsesFeature represents a <uses-feature> element.
type ManifestUsesFeature struct {
	Name     string // android:name
	Required string // android:required value ("true", "false", or "")
	Line     int
}

// ManifestMetaData represents a <meta-data> element within a component.
type ManifestMetaData struct {
	Name     string // android:name
	Value    string // android:value
	Resource string // android:resource
}

// ManifestComponent represents an activity, service, receiver, or provider.
type ManifestComponent struct {
	Tag                     string // "activity", "service", "receiver", "provider"
	Name                    string // android:name
	Line                    int
	Exported                *bool  // nil = not set
	Permission              string // android:permission
	HasIntentFilter         bool
	ParentTag               string             // tag name of the parent element
	IntentFilterActions     []string           // action android:name values from all intent-filters
	IntentFilterCategories  []string           // category android:name values from all intent-filters
	IntentFilterDataSchemes []string           // data android:scheme values from all intent-filters
	IntentFilterDataHosts   []string           // data android:host values from all intent-filters
	MetaDataEntries         []ManifestMetaData // <meta-data> children
}

// ManifestElement represents any element in the manifest with its position.
type ManifestElement struct {
	Tag       string
	Line      int
	ParentTag string
}

// ---------------------------------------------------------------------------
// Manifest-rule structural contract
// ---------------------------------------------------------------------------

// ManifestFamily is the structural type a rule must satisfy to be
// registered via RegisterManifest and stored in ManifestRules. It is
// defined as a type alias for an anonymous interface so external
// callers interact with it by method set, not by a named family
// interface. (Replaces the old `manifest rule` interface.)
type ManifestFamily = interface {
	Rule
	AndroidDependencyProvider
	CheckManifest(m *Manifest) []scanner.Finding
}

// ManifestBase provides default nil implementations for Rule methods that
// don't apply to manifest rules (Check, CheckNode, CheckLines).
type ManifestBase struct{}

func (ManifestBase) Check(file *scanner.File) []scanner.Finding { return nil }
func (ManifestBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepManifest
}

// ---------------------------------------------------------------------------
// Helper to create a manifest finding (no scanner.File needed)
// ---------------------------------------------------------------------------

func manifestFinding(path string, line int, rule BaseRule, msg string) scanner.Finding {
	return scanner.Finding{
		File:     path,
		Line:     line,
		Col:      1,
		RuleSet:  rule.RuleSetName,
		Rule:     rule.RuleName,
		Severity: rule.Sev,
		Message:  msg,
	}
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// allComponents returns all components from the application element.
func allComponents(app *ManifestApplication) []ManifestComponent {
	var all []ManifestComponent
	all = append(all, app.Activities...)
	all = append(all, app.Services...)
	all = append(all, app.Receivers...)
	all = append(all, app.Providers...)
	return all
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

// ManifestRules holds all registered manifest rules for use by the scanner.
var ManifestRules []ManifestFamily

// RegisterManifest adds a manifest rule to both the manifest registry and the
// global rule registry (for config/suppression compatibility).
func RegisterManifest(r ManifestFamily) {
	ManifestRules = append(ManifestRules, r)
	Register(r)
}
