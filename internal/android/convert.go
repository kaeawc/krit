package android

import (
	"strconv"
	"strings"
)

// ManifestForRules holds the data needed by manifest rule implementations.
// This mirrors rules.Manifest but lives in the android package to avoid
// a circular import. The main.go conversion uses field-by-field copy.
//
// See rules.Manifest for the canonical struct used by manifest rule.

// ConvertedManifest holds the intermediate data extracted from parsing that
// main.go can use to populate rules.Manifest without circular imports.
type ConvertedManifest struct {
	Path        string
	Package     string
	MinSDK      int
	TargetSDK   int
	HasUsesSdk  bool
	UsesSdkLine int

	UsesPermissions []string
	Permissions     []string
	UsesFeatures    []ConvertedUsesFeature

	// Application fields
	HasApplication        bool
	AppLine               int
	AllowBackup           *bool
	Debuggable            *bool
	LocaleConfig          string
	SupportsRtl           *bool
	ExtractNativeLibs     *bool
	Icon                  string
	UsesCleartextTraffic  *bool
	FullBackupContent     string
	DataExtractionRules   string
	NetworkSecurityConfig string

	// Components
	Activities []ConvertedComponent
	Services   []ConvertedComponent
	Receivers  []ConvertedComponent
	Providers  []ConvertedComponent

	// All elements for WrongManifestParent checks
	Elements []ConvertedElement
}

// ConvertedUsesFeature holds a <uses-feature> entry for rules.
type ConvertedUsesFeature struct {
	Name     string
	Required string
	Line     int
}

// ConvertedMetaData holds a single meta-data entry for rules.
type ConvertedMetaData struct {
	Name     string
	Value    string
	Resource string
}

// ConvertedComponent holds component data for rules.
type ConvertedComponent struct {
	Tag                     string
	Name                    string
	Line                    int
	Exported                *bool
	Permission              string
	HasIntentFilter         bool
	ParentTag               string
	IntentFilterActions     []string
	IntentFilterCategories  []string
	IntentFilterDataSchemes []string
	MetaDataEntries         []ConvertedMetaData
}

// ConvertedElement holds element data for rules.
type ConvertedElement struct {
	Tag       string
	Line      int
	ParentTag string
}

// ConvertManifest converts a parsed android.Manifest into the intermediate
// format that main.go uses to build rules.Manifest.
func ConvertManifest(m *Manifest, path string) *ConvertedManifest {
	cm := &ConvertedManifest{
		Path:            path,
		Package:         m.Package,
		UsesPermissions: make([]string, 0, len(m.UsesPermissions)),
		Permissions:     make([]string, 0, len(m.Permissions)),
	}
	convertSDKVersions(m, cm)
	convertPermissions(m, cm)
	convertElements(m, cm)
	convertApplication(m, cm)
	return cm
}

func convertSDKVersions(m *Manifest, cm *ConvertedManifest) {
	if m.UsesSdk.MinSdkVersion != "" {
		cm.MinSDK, _ = strconv.Atoi(m.UsesSdk.MinSdkVersion)
	}
	if m.UsesSdk.TargetSdkVersion != "" {
		cm.TargetSDK, _ = strconv.Atoi(m.UsesSdk.TargetSdkVersion)
	}
	if m.UsesSdk.MinSdkVersion != "" || m.UsesSdk.TargetSdkVersion != "" {
		cm.HasUsesSdk = true
		cm.UsesSdkLine = 1
	}
}

func convertPermissions(m *Manifest, cm *ConvertedManifest) {
	for _, perm := range m.UsesPermissions {
		if perm.Name == "" {
			continue
		}
		// tools:node="remove"/"removeAll" instructs the manifest merger
		// to drop a permission contributed by a library — the app is not
		// requesting it.
		if perm.ToolsNode == ToolsNodeRemove || perm.ToolsNode == ToolsNodeRemoveAll {
			continue
		}
		cm.UsesPermissions = append(cm.UsesPermissions, perm.Name)
	}
	for _, perm := range m.Permissions {
		if perm.Name == "" {
			continue
		}
		cm.Permissions = append(cm.Permissions, perm.Name)
	}
	for _, f := range m.UsesFeatures {
		if f.Name == "" {
			continue
		}
		cm.UsesFeatures = append(cm.UsesFeatures, ConvertedUsesFeature{
			Name:     f.Name,
			Required: f.Required,
		})
	}
}

func convertElements(m *Manifest, cm *ConvertedManifest) {
	for _, elem := range m.Elements {
		cm.Elements = append(cm.Elements, ConvertedElement{
			Tag:       elem.Tag,
			Line:      elem.Line,
			ParentTag: elem.ParentTag,
		})
		if elem.Tag == "uses-sdk" && elem.ParentTag == "manifest" && !cm.HasUsesSdk {
			cm.HasUsesSdk = true
			cm.UsesSdkLine = elem.Line
		}
		if elem.Tag == "uses-sdk" && elem.ParentTag == "manifest" {
			cm.UsesSdkLine = elem.Line
		}
		if elem.Tag == "application" && elem.ParentTag == "manifest" && cm.AppLine == 0 {
			cm.AppLine = elem.Line
		}
	}
}

func convertApplication(m *Manifest, cm *ConvertedManifest) {
	app := &m.Application
	if app.Name == "" && len(app.Activities) == 0 && len(app.Services) == 0 &&
		len(app.Receivers) == 0 && len(app.Providers) == 0 && app.Debuggable == "" &&
		app.AllowBackup == "" && app.LocaleConfig == "" &&
		app.SupportsRtl == "" && app.ExtractNativeLibs == "" &&
		app.Theme == "" && app.Icon == "" &&
		app.UsesCleartextTraffic == "" && app.NetworkSecurityConfig == "" &&
		app.FullBackupContent == "" && app.DataExtractionRules == "" && len(app.MetaData) == 0 {
		return
	}
	cm.HasApplication = true
	if cm.AppLine == 0 {
		cm.AppLine = 1
	}
	cm.LocaleConfig = app.LocaleConfig
	cm.Icon = app.Icon
	cm.UsesCleartextTraffic = parseBoolPtr(app.UsesCleartextTraffic)
	cm.FullBackupContent = app.FullBackupContent
	cm.DataExtractionRules = app.DataExtractionRules
	cm.NetworkSecurityConfig = app.NetworkSecurityConfig
	cm.AllowBackup = parseBoolPtr(app.AllowBackup)
	cm.Debuggable = parseBoolPtr(app.Debuggable)
	cm.SupportsRtl = parseBoolPtr(app.SupportsRtl)
	cm.ExtractNativeLibs = parseBoolPtr(app.ExtractNativeLibs)
	convertActivityComponents(m, app, cm)
	convertServiceComponents(m, app, cm)
	convertReceiverComponents(m, app, cm)
	convertProviderComponents(m, app, cm)
}

func convertActivityComponents(m *Manifest, app *Application, cm *ConvertedManifest) {
	activityNodes := childrenByTag(applicationNode(m.Root), "activity")
	for i, a := range app.Activities {
		actions, categories, schemes := extractIntentFilterDetails(a.IntentFilters)
		cm.Activities = append(cm.Activities, ConvertedComponent{
			Tag:                     "activity",
			Name:                    a.Name,
			Line:                    lineForNodeAt(activityNodes, i),
			Exported:                parseBoolPtr(a.Exported),
			Permission:              a.Permission,
			HasIntentFilter:         len(a.IntentFilters) > 0,
			ParentTag:               "application",
			IntentFilterActions:     actions,
			IntentFilterCategories:  categories,
			IntentFilterDataSchemes: schemes,
		})
	}
}

func convertServiceComponents(m *Manifest, app *Application, cm *ConvertedManifest) {
	serviceNodes := childrenByTag(applicationNode(m.Root), "service")
	for i, s := range app.Services {
		actions, categories, schemes := extractIntentFilterDetails(s.IntentFilters)
		cm.Services = append(cm.Services, ConvertedComponent{
			Tag:                     "service",
			Name:                    s.Name,
			Line:                    lineForNodeAt(serviceNodes, i),
			Exported:                parseBoolPtr(s.Exported),
			Permission:              s.Permission,
			HasIntentFilter:         len(s.IntentFilters) > 0,
			ParentTag:               "application",
			IntentFilterActions:     actions,
			IntentFilterCategories:  categories,
			IntentFilterDataSchemes: schemes,
		})
	}
}

func convertReceiverComponents(m *Manifest, app *Application, cm *ConvertedManifest) {
	receiverNodes := childrenByTag(applicationNode(m.Root), "receiver")
	for i, r := range app.Receivers {
		actions, categories, schemes := extractIntentFilterDetails(r.IntentFilters)
		var metaEntries []ConvertedMetaData
		for _, md := range r.MetaData {
			metaEntries = append(metaEntries, ConvertedMetaData(md))
		}
		cm.Receivers = append(cm.Receivers, ConvertedComponent{
			Tag:                     "receiver",
			Name:                    r.Name,
			Line:                    lineForNodeAt(receiverNodes, i),
			Exported:                parseBoolPtr(r.Exported),
			Permission:              r.Permission,
			HasIntentFilter:         len(r.IntentFilters) > 0,
			ParentTag:               "application",
			IntentFilterActions:     actions,
			IntentFilterCategories:  categories,
			IntentFilterDataSchemes: schemes,
			MetaDataEntries:         metaEntries,
		})
	}
}

func convertProviderComponents(m *Manifest, app *Application, cm *ConvertedManifest) {
	providerNodes := childrenByTag(applicationNode(m.Root), "provider")
	for i, p := range app.Providers {
		cm.Providers = append(cm.Providers, ConvertedComponent{
			Tag:        "provider",
			Name:       p.Name,
			Line:       lineForNodeAt(providerNodes, i),
			Exported:   parseBoolPtr(p.Exported),
			Permission: p.Permission,
			ParentTag:  "application",
		})
	}
}

func parseBoolPtr(s string) *bool {
	if s == "" {
		return nil
	}
	b := strings.EqualFold(s, "true")
	return &b
}

func applicationNode(root *XMLNode) *XMLNode {
	if root == nil {
		return nil
	}
	return root.ChildByTag("application")
}

func childrenByTag(parent *XMLNode, tag string) []*XMLNode {
	if parent == nil {
		return nil
	}
	return parent.ChildrenByTag(tag)
}

func lineForNodeAt(nodes []*XMLNode, idx int) int {
	if idx >= 0 && idx < len(nodes) && nodes[idx] != nil {
		return nodes[idx].Line
	}
	return 1
}

// extractIntentFilterDetails collects all action names, category names, and
// data schemes from a slice of IntentFilters.
func extractIntentFilterDetails(filters []IntentFilter) (actions, categories, schemes []string) {
	for _, f := range filters {
		for _, a := range f.Actions {
			if a.Name != "" {
				actions = append(actions, a.Name)
			}
		}
		for _, c := range f.Categories {
			if c.Name != "" {
				categories = append(categories, c.Name)
			}
		}
		for _, d := range f.Data {
			if d.Scheme != "" {
				schemes = append(schemes, d.Scheme)
			}
		}
	}
	return
}
