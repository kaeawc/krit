// Package manifest carries the rule-facing data model for parsed
// AndroidManifest.xml files.
//
// It exists as a leaf package so that the v2 dispatcher (internal/rules/v2)
// can declare a strongly typed Manifest field on Context without depending
// on the rules package — which would create an import cycle. The pipeline
// converts an XML-parsed android.Manifest into a *manifest.Manifest and
// hands it to the dispatcher; rule Check functions read it through
// ctx.Manifest directly with no type assertion.
package manifest

// Manifest represents a parsed AndroidManifest.xml.
type Manifest struct {
	Path            string   // file path to AndroidManifest.xml
	Package         string   // package attribute on <manifest>
	MinSDK          int      // android:minSdkVersion (0 if absent)
	TargetSDK       int      // android:targetSdkVersion (0 if absent)
	VersionCode     string   // android:versionCode on <manifest> ("" if absent)
	VersionName     string   // android:versionName on <manifest> ("" if absent)
	UsesSdk         *Element // <uses-sdk> element, nil if missing
	Application     *Application
	Elements        []Element     // all top-level children of <manifest>
	UsesPermissions []string      // <uses-permission android:name="..."> values
	Permissions     []string      // <permission android:name="..."> values
	UsesFeatures    []UsesFeature // <uses-feature> elements
	IsDebugManifest bool          // true if this is a debug build-variant manifest
	HasNativeLibs   bool          // true if the project contains .so native libraries
}

// Application represents the <application> element.
type Application struct {
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
	Activities            []Component
	Services              []Component
	Receivers             []Component
	Providers             []Component
}

// UsesFeature represents a <uses-feature> element.
type UsesFeature struct {
	Name     string // android:name
	Required string // android:required value ("true", "false", or "")
	Line     int
}

// MetaData represents a <meta-data> element within a component.
type MetaData struct {
	Name     string // android:name
	Value    string // android:value
	Resource string // android:resource
}

// Component represents an activity, service, receiver, or provider.
type Component struct {
	Tag                     string // "activity", "service", "receiver", "provider"
	Name                    string // android:name
	Line                    int
	Exported                *bool  // nil = not set
	Permission              string // android:permission
	HasIntentFilter         bool
	ParentTag               string     // tag name of the parent element
	IntentFilterActions     []string   // action android:name values from all intent-filters
	IntentFilterCategories  []string   // category android:name values from all intent-filters
	IntentFilterDataSchemes []string   // data android:scheme values from all intent-filters
	IntentFilterDataHosts   []string   // data android:host values from all intent-filters
	MetaDataEntries         []MetaData // <meta-data> children
}

// Element represents any element in the manifest with its position.
type Element struct {
	Tag       string
	Line      int
	ParentTag string
}

// AllComponents returns the merged list of activities, services, receivers,
// and providers declared on the application element.
func AllComponents(app *Application) []Component {
	if app == nil {
		return nil
	}
	all := make([]Component, 0, len(app.Activities)+len(app.Services)+len(app.Receivers)+len(app.Providers))
	all = append(all, app.Activities...)
	all = append(all, app.Services...)
	all = append(all, app.Receivers...)
	all = append(all, app.Providers...)
	return all
}
