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
	Elements        []Element // all top-level children of <manifest>
	UsesPermissions []string  // <uses-permission android:name="..."> values
	Permissions     []string  // <permission android:name="..."> values
	// UsesPermissionMaxSdk maps a <uses-permission> name to its android:maxSdkVersion, when set.
	UsesPermissionMaxSdk map[string]int
	// UsesPermissionEntries lists every <uses-permission> element in document order, including repeats of one name.
	UsesPermissionEntries []UsesPermissionEntry
	// UsesPermissionLines and PermissionLines map a permission name to the line of its first element.
	UsesPermissionLines map[string]int
	PermissionLines     map[string]int
	// ToolsIgnoreByLine lists the tools:ignore tokens in effect for the element starting at each line, including inherited ones.
	ToolsIgnoreByLine map[int][]string
	// RootLine is the line of the <manifest> element; findings anchored at line 1 use its tools:ignore tokens.
	RootLine        int
	UsesFeatures    []UsesFeature // <uses-feature> elements
	IsDebugManifest bool          // true if this is a debug build-variant manifest
	HasNativeLibs   bool          // true if the project contains .so native libraries
	// ActivityExtendsPreferenceActivity maps a source-declared class FQN to whether its
	// superclass chain contains android.preference.PreferenceActivity. Classes absent
	// from the map are not declared in project sources.
	ActivityExtendsPreferenceActivity map[string]bool
	// Root is the full element tree of the manifest, for checks that need elements the typed fields do not carry.
	Root *Node
}

// Node is a position-aware manifest element.
type Node struct {
	Tag      string
	Line     int
	Col      int
	Attrs    []Attr
	Children []*Node
}

// Attr is a position-aware manifest attribute. Name keeps its prefix, as in "android:name".
type Attr struct {
	Name  string
	Value string
	Line  int
	Col   int
	// ValueCol is the 1-based column where the value text starts, after the opening quote.
	ValueCol int
}

// Attr returns the attribute with the given prefixed name.
func (n *Node) Attr(name string) (Attr, bool) {
	if n == nil {
		return Attr{}, false
	}
	for _, a := range n.Attrs {
		if a.Name == name {
			return a, true
		}
	}
	return Attr{}, false
}

// AttrValue returns the value of the attribute with the given prefixed name, or "".
func (n *Node) AttrValue(name string) string {
	a, _ := n.Attr(name)
	return a.Value
}

// ChildrenByTag returns the direct children with the given tag.
func (n *Node) ChildrenByTag(tag string) []*Node {
	var out []*Node
	if n == nil {
		return out
	}
	for _, c := range n.Children {
		if c.Tag == tag {
			out = append(out, c)
		}
	}
	return out
}

// Find returns the first descendant, depth first, with the given tag, or nil.
func (n *Node) Find(tag string) *Node {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Tag == tag {
			return c
		}
		if found := c.Find(tag); found != nil {
			return found
		}
	}
	return nil
}

// Application represents the <application> element.
type Application struct {
	Line                  int
	ToolsNode             string // tools:node merge marker on <application>
	AllowBackup           *bool  // nil = not set, true/false = explicit
	Debuggable            *bool  // nil = not set
	LocaleConfig          string // android:localeConfig value
	Icon                  string // android:icon value
	HasBanner             bool   // android:banner is set
	UsesCleartextTraffic  *bool  // nil = not set, true/false = explicit
	FullBackupContent     string // android:fullBackupContent value
	DataExtractionRules   string // android:dataExtractionRules value
	SupportsRtl           *bool  // nil = not set, true/false = explicit
	ExtractNativeLibs     *bool  // nil = not set, true/false = explicit
	NetworkSecurityConfig string // android:networkSecurityConfig value
	Activities            []Component
	LauncherAliasTargets  []string // android:targetActivity of <activity-alias> elements with a LAUNCHER filter
	Services              []Component
	Receivers             []Component
	Providers             []Component
}

// UsesFeature represents a <uses-feature> element.
type UsesFeature struct {
	Name     string // android:name
	Required string // android:required value ("true", "false", or "")
	Line     int
	// GlEsVersion is android:glEsVersion for a feature declared without a name.
	GlEsVersion string
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
	// ExportedRaw is the android:exported attribute text, empty when the attribute is absent.
	ExportedRaw string
	// HasBanner is true when the element sets android:banner.
	HasBanner bool
	// HasNavGraph is true when the element has a <nav-graph> child.
	HasNavGraph bool
	// FirstFilterAction and FirstFilterCategory are the first action and category of the first intent-filter.
	FirstFilterAction   string
	FirstFilterCategory string
}

// UsesPermissionEntry is one <uses-permission> element.
type UsesPermissionEntry struct {
	Name      string
	MaxSdk    int // android:maxSdkVersion, valid when HasMaxSdk
	HasMaxSdk bool
	Line      int
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
