// Package android provides parsers for Android project files.
package android

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
)

// Manifest represents a parsed AndroidManifest.xml.
type Manifest struct {
	Package         string           `xml:"package,attr"`
	VersionCode     string           `xml:"http://schemas.android.com/apk/res/android versionCode,attr"`
	VersionName     string           `xml:"http://schemas.android.com/apk/res/android versionName,attr"`
	UsesSdk         UsesSdk          `xml:"uses-sdk"`
	UsesPermissions []UsesPermission `xml:"uses-permission"`
	Permissions     []Permission     `xml:"permission"`
	Application     Application      `xml:"application"`
	Queries         []Intent         `xml:"queries>intent"`
	UsesFeatures    []UsesFeature    `xml:"uses-feature"`
	Elements        []ElementInfo    `xml:"-"`
	Root            *XMLNode         `xml:"-"`
}

// UsesSdk holds minSdkVersion and targetSdkVersion.
type UsesSdk struct {
	MinSdkVersion    string `xml:"http://schemas.android.com/apk/res/android minSdkVersion,attr"`
	TargetSdkVersion string `xml:"http://schemas.android.com/apk/res/android targetSdkVersion,attr"`
}

// UsesPermission is a <uses-permission> element.
type UsesPermission struct {
	Name string `xml:"http://schemas.android.com/apk/res/android name,attr"`
}

// Permission is a <permission> element.
type Permission struct {
	Name            string `xml:"http://schemas.android.com/apk/res/android name,attr"`
	ProtectionLevel string `xml:"http://schemas.android.com/apk/res/android protectionLevel,attr"`
}

// UsesFeature is a <uses-feature> element.
type UsesFeature struct {
	Name     string `xml:"http://schemas.android.com/apk/res/android name,attr"`
	Required string `xml:"http://schemas.android.com/apk/res/android required,attr"`
}

// Application holds application-level attributes and components.
type Application struct {
	Name                  string     `xml:"http://schemas.android.com/apk/res/android name,attr"`
	Debuggable            string     `xml:"http://schemas.android.com/apk/res/android debuggable,attr"`
	AllowBackup           string     `xml:"http://schemas.android.com/apk/res/android allowBackup,attr"`
	LocaleConfig          string     `xml:"http://schemas.android.com/apk/res/android localeConfig,attr"`
	SupportsRtl           string     `xml:"http://schemas.android.com/apk/res/android supportsRtl,attr"`
	ExtractNativeLibs     string     `xml:"http://schemas.android.com/apk/res/android extractNativeLibs,attr"`
	UsesCleartextTraffic  string     `xml:"http://schemas.android.com/apk/res/android usesCleartextTraffic,attr"`
	NetworkSecurityConfig string     `xml:"http://schemas.android.com/apk/res/android networkSecurityConfig,attr"`
	FullBackupContent     string     `xml:"http://schemas.android.com/apk/res/android fullBackupContent,attr"`
	DataExtractionRules   string     `xml:"http://schemas.android.com/apk/res/android dataExtractionRules,attr"`
	Theme                 string     `xml:"http://schemas.android.com/apk/res/android theme,attr"`
	Label                 string     `xml:"http://schemas.android.com/apk/res/android label,attr"`
	Icon                  string     `xml:"http://schemas.android.com/apk/res/android icon,attr"`
	Activities            []Activity `xml:"activity"`
	Services              []Service  `xml:"service"`
	Receivers             []Receiver `xml:"receiver"`
	Providers             []Provider `xml:"provider"`
	MetaData              []MetaData `xml:"meta-data"`
}

// ElementInfo captures raw XML element names and parents for manifest-oriented lint rules.
type ElementInfo struct {
	Tag       string
	ParentTag string
	Line      int
}

// Component is the shared interface for all Android manifest components.
type Component struct {
	Name       string
	Exported   string // "true", "false", or "" (unset)
	Permission string
}

// Activity is an <activity> element.
type Activity struct {
	Name          string         `xml:"http://schemas.android.com/apk/res/android name,attr"`
	Exported      string         `xml:"http://schemas.android.com/apk/res/android exported,attr"`
	Permission    string         `xml:"http://schemas.android.com/apk/res/android permission,attr"`
	IntentFilters []IntentFilter `xml:"intent-filter"`
}

// Service is a <service> element.
type Service struct {
	Name          string         `xml:"http://schemas.android.com/apk/res/android name,attr"`
	Exported      string         `xml:"http://schemas.android.com/apk/res/android exported,attr"`
	Permission    string         `xml:"http://schemas.android.com/apk/res/android permission,attr"`
	IntentFilters []IntentFilter `xml:"intent-filter"`
}

// Receiver is a <receiver> element.
type Receiver struct {
	Name          string         `xml:"http://schemas.android.com/apk/res/android name,attr"`
	Exported      string         `xml:"http://schemas.android.com/apk/res/android exported,attr"`
	Permission    string         `xml:"http://schemas.android.com/apk/res/android permission,attr"`
	IntentFilters []IntentFilter `xml:"intent-filter"`
	MetaData      []MetaData     `xml:"meta-data"`
}

// Provider is a <provider> element.
type Provider struct {
	Name        string `xml:"http://schemas.android.com/apk/res/android name,attr"`
	Exported    string `xml:"http://schemas.android.com/apk/res/android exported,attr"`
	Permission  string `xml:"http://schemas.android.com/apk/res/android permission,attr"`
	Authorities string `xml:"http://schemas.android.com/apk/res/android authorities,attr"`
}

// IntentFilter is an <intent-filter> element.
type IntentFilter struct {
	Actions    []IntentAction   `xml:"action"`
	Categories []IntentCategory `xml:"category"`
	Data       []IntentData     `xml:"data"`
}

// IntentAction is an <action> within an intent-filter.
type IntentAction struct {
	Name string `xml:"http://schemas.android.com/apk/res/android name,attr"`
}

// IntentCategory is a <category> within an intent-filter.
type IntentCategory struct {
	Name string `xml:"http://schemas.android.com/apk/res/android name,attr"`
}

// IntentData is a <data> element within an intent-filter.
type IntentData struct {
	Scheme string `xml:"http://schemas.android.com/apk/res/android scheme,attr"`
	Host   string `xml:"http://schemas.android.com/apk/res/android host,attr"`
}

// Intent is a <intent> element (used inside <queries>).
type Intent struct {
	Actions []IntentAction `xml:"action"`
}

// MetaData is a <meta-data> element.
type MetaData struct {
	Name     string `xml:"http://schemas.android.com/apk/res/android name,attr"`
	Value    string `xml:"http://schemas.android.com/apk/res/android value,attr"`
	Resource string `xml:"http://schemas.android.com/apk/res/android resource,attr"`
}

// ParseManifest reads and parses an AndroidManifest.xml file.
func ParseManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", path, err)
	}
	return ParseManifestBytes(data)
}

// ParseManifestBytes parses AndroidManifest.xml content from bytes.
func ParseManifestBytes(data []byte) (*Manifest, error) {
	var m Manifest
	if err := xml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest XML: %w", err)
	}
	root, err := ParseXMLAST(data)
	if err != nil {
		return nil, fmt.Errorf("parsing manifest tree-sitter AST: %w", err)
	}
	m.Root = root
	m.Elements = flattenElementInfo(root, "")
	return &m, nil
}

func flattenElementInfo(node *XMLNode, parentTag string) []ElementInfo {
	if node == nil {
		return nil
	}
	elements := []ElementInfo{{
		Tag:       node.Tag,
		ParentTag: parentTag,
		Line:      node.Line,
	}}
	for _, child := range node.Children {
		elements = append(elements, flattenElementInfo(child, node.Tag)...)
	}
	return elements
}

// IsExported returns true if a component is exported. A component is considered
// exported if android:exported="true", or if exported is unset and the component
// has at least one intent-filter (the pre-API-31 default behavior).
func IsExported(c Component, hasIntentFilters bool) bool {
	switch strings.ToLower(c.Exported) {
	case "true":
		return true
	case "false":
		return false
	default:
		// If exported is not explicitly set, components with intent-filters
		// were implicitly exported before API 31.
		return hasIntentFilters
	}
}

// HasPermission returns true if a component declares android:permission.
func HasPermission(c Component) bool {
	return c.Permission != ""
}

// componentOf converts a typed component to the generic Component struct.
func activityComponent(a *Activity) Component {
	return Component{Name: a.Name, Exported: a.Exported, Permission: a.Permission}
}

func serviceComponent(s *Service) Component {
	return Component{Name: s.Name, Exported: s.Exported, Permission: s.Permission}
}

func receiverComponent(r *Receiver) Component {
	return Component{Name: r.Name, Exported: r.Exported, Permission: r.Permission}
}

func providerComponent(p *Provider) Component {
	return Component{Name: p.Name, Exported: p.Exported, Permission: p.Permission}
}

// ComponentResult wraps a found component with its type information.
type ComponentResult struct {
	Component
	Type          string // "activity", "service", "receiver", "provider"
	IntentFilters []IntentFilter
	Authorities   string // only for providers
}

// FindComponent searches all component types by class name. The name can be
// a simple class name (e.g. "MainActivity"), a relative name (e.g. ".MainActivity"),
// or a fully qualified name (e.g. "com.example.MainActivity").
func (m *Manifest) FindComponent(name string) *ComponentResult {
	for i := range m.Application.Activities {
		a := &m.Application.Activities[i]
		if matchComponentName(m.Package, a.Name, name) {
			return &ComponentResult{
				Component:     activityComponent(a),
				Type:          "activity",
				IntentFilters: a.IntentFilters,
			}
		}
	}
	for i := range m.Application.Services {
		s := &m.Application.Services[i]
		if matchComponentName(m.Package, s.Name, name) {
			return &ComponentResult{
				Component:     serviceComponent(s),
				Type:          "service",
				IntentFilters: s.IntentFilters,
			}
		}
	}
	for i := range m.Application.Receivers {
		r := &m.Application.Receivers[i]
		if matchComponentName(m.Package, r.Name, name) {
			return &ComponentResult{
				Component:     receiverComponent(r),
				Type:          "receiver",
				IntentFilters: r.IntentFilters,
			}
		}
	}
	for i := range m.Application.Providers {
		p := &m.Application.Providers[i]
		if matchComponentName(m.Package, p.Name, name) {
			return &ComponentResult{
				Component:   providerComponent(p),
				Type:        "provider",
				Authorities: p.Authorities,
			}
		}
	}
	return nil
}

// AllComponents returns every component in the manifest as ComponentResults.
func (m *Manifest) AllComponents() []ComponentResult {
	var results []ComponentResult
	for i := range m.Application.Activities {
		a := &m.Application.Activities[i]
		results = append(results, ComponentResult{
			Component:     activityComponent(a),
			Type:          "activity",
			IntentFilters: a.IntentFilters,
		})
	}
	for i := range m.Application.Services {
		s := &m.Application.Services[i]
		results = append(results, ComponentResult{
			Component:     serviceComponent(s),
			Type:          "service",
			IntentFilters: s.IntentFilters,
		})
	}
	for i := range m.Application.Receivers {
		r := &m.Application.Receivers[i]
		results = append(results, ComponentResult{
			Component:     receiverComponent(r),
			Type:          "receiver",
			IntentFilters: r.IntentFilters,
		})
	}
	for i := range m.Application.Providers {
		p := &m.Application.Providers[i]
		results = append(results, ComponentResult{
			Component:   providerComponent(p),
			Type:        "provider",
			Authorities: p.Authorities,
		})
	}
	return results
}

// ExportedWithoutPermission returns components that are exported but have no
// android:permission set. Useful for security lint checks.
func (m *Manifest) ExportedWithoutPermission() []ComponentResult {
	var results []ComponentResult
	for _, c := range m.AllComponents() {
		hasFilters := len(c.IntentFilters) > 0
		if IsExported(c.Component, hasFilters) && !HasPermission(c.Component) {
			results = append(results, c)
		}
	}
	return results
}

// matchComponentName checks if a declared component name matches a query.
// Handles fully qualified, relative (.Name), and simple name lookups.
func matchComponentName(pkg, declared, query string) bool {
	if declared == "" {
		return false
	}
	// Resolve relative names to fully qualified.
	fqn := declared
	if strings.HasPrefix(declared, ".") {
		fqn = pkg + declared
	}

	// Exact match on fully qualified name.
	if fqn == query {
		return true
	}
	// Match on the declared attribute exactly.
	if declared == query {
		return true
	}
	// Simple name match: compare last segment after the final dot.
	idx := strings.LastIndex(fqn, ".")
	if idx >= 0 && fqn[idx+1:] == query {
		return true
	}
	return false
}
