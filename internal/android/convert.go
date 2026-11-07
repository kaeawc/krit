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
	// UsesPermissionEntries lists every <uses-permission> element in document order.
	UsesPermissionEntries []ConvertedUsesPermission
	// UsesPermissionMaxSdk holds android:maxSdkVersion for permissions that set it.
	UsesPermissionMaxSdk map[string]int
	// UsesPermissionLines and PermissionLines map a permission name to the line of its
	// first <uses-permission> or <permission> element under <manifest>.
	UsesPermissionLines map[string]int
	PermissionLines     map[string]int
	// ToolsIgnoreByLine lists the tools:ignore tokens in effect for the element starting at each line, including those inherited from ancestors.
	ToolsIgnoreByLine map[int][]string
	Permissions       []string
	// RootLine is the line of the <manifest> element.
	RootLine     int
	UsesFeatures []ConvertedUsesFeature

	// Application fields
	HasApplication bool
	AppLine        int
	// AppToolsNode is the tools:node merge marker on <application>.
	AppToolsNode          string
	AllowBackup           *bool
	Debuggable            *bool
	LocaleConfig          string
	SupportsRtl           *bool
	ExtractNativeLibs     *bool
	Icon                  string
	HasBanner             bool
	UsesCleartextTraffic  *bool
	FullBackupContent     string
	DataExtractionRules   string
	NetworkSecurityConfig string

	// LauncherAliasTargets lists activities reached through an
	// <activity-alias> that has a LAUNCHER intent-filter.
	LauncherAliasTargets []string

	// Components
	Activities []ConvertedComponent
	Services   []ConvertedComponent
	Receivers  []ConvertedComponent
	Providers  []ConvertedComponent

	// All elements for WrongManifestParent checks
	Elements []ConvertedElement

	// Root is the parsed element tree.
	Root *XMLNode
}

// ConvertedUsesFeature holds a <uses-feature> entry for rules.
type ConvertedUsesFeature struct {
	Name     string
	Required string
	Line     int
	// GlEsVersion is the android:glEsVersion of a feature declared without a name.
	GlEsVersion string
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
	IntentFilterDataHosts   []string
	MetaDataEntries         []ConvertedMetaData
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

// ConvertedUsesPermission is one <uses-permission> element.
type ConvertedUsesPermission struct {
	Name      string
	MaxSdk    int
	HasMaxSdk bool
	Line      int
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
	convertToolsIgnores(m, cm)
	cm.Root = m.Root
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
	// Typed permissions and manifest-level elements appear in the same document order.
	var usesLines, permLines, featureLines []int
	for _, elem := range m.Elements {
		if elem.ParentTag != "manifest" {
			continue
		}
		switch elem.Tag {
		case "uses-permission":
			usesLines = append(usesLines, elem.Line)
		case "permission":
			permLines = append(permLines, elem.Line)
		case "uses-feature":
			featureLines = append(featureLines, elem.Line)
		}
	}
	firstLine := func(dst *map[string]int, lines []int, i int, name string) {
		if i >= len(lines) {
			return
		}
		if *dst == nil {
			*dst = make(map[string]int)
		}
		if _, ok := (*dst)[name]; !ok {
			(*dst)[name] = lines[i]
		}
	}
	for i, perm := range m.UsesPermissions {
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
		firstLine(&cm.UsesPermissionLines, usesLines, i, perm.Name)
		entry := ConvertedUsesPermission{Name: perm.Name}
		if i < len(usesLines) {
			entry.Line = usesLines[i]
		}
		if maxSdk, err := strconv.Atoi(perm.MaxSdkVersion); err == nil {
			entry.MaxSdk, entry.HasMaxSdk = maxSdk, true
		}
		cm.UsesPermissionEntries = append(cm.UsesPermissionEntries, entry)
		if maxSdk, err := strconv.Atoi(perm.MaxSdkVersion); err == nil {
			if cm.UsesPermissionMaxSdk == nil {
				cm.UsesPermissionMaxSdk = make(map[string]int)
			}
			cm.UsesPermissionMaxSdk[perm.Name] = maxSdk
		}
	}
	for i, perm := range m.Permissions {
		if perm.Name == "" {
			continue
		}
		firstLine(&cm.PermissionLines, permLines, i, perm.Name)
		cm.Permissions = append(cm.Permissions, perm.Name)
	}
	for i, f := range m.UsesFeatures {
		if f.Name == "" && f.GlEsVersion == "" {
			continue
		}
		line := 0
		if i < len(featureLines) {
			line = featureLines[i]
		}
		cm.UsesFeatures = append(cm.UsesFeatures, ConvertedUsesFeature{
			Name:        f.Name,
			Required:    f.Required,
			Line:        line,
			GlEsVersion: f.GlEsVersion,
		})
	}
}

// convertToolsIgnores records the tools:ignore tokens that apply to each element, inherited from its ancestors.
func convertToolsIgnores(m *Manifest, cm *ConvertedManifest) {
	var walk func(n *XMLNode, inherited []string)
	walk = func(n *XMLNode, inherited []string) {
		if n == nil {
			return
		}
		tokens := inherited
		if raw := n.Attr("tools:ignore"); raw != "" {
			tokens = append(append([]string(nil), inherited...), ToolsIgnoreTokens(raw)...)
		}
		if len(tokens) > 0 && n.Line > 0 {
			if cm.ToolsIgnoreByLine == nil {
				cm.ToolsIgnoreByLine = make(map[int][]string)
			}
			cm.ToolsIgnoreByLine[n.Line] = append(cm.ToolsIgnoreByLine[n.Line], tokens...)
		}
		for _, child := range n.Children {
			walk(child, tokens)
		}
	}
	walk(m.Root, nil)
	if m.Root != nil {
		cm.RootLine = m.Root.Line
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
	if app.Name == "" && app.ToolsNode == "" && len(app.Activities) == 0 && len(app.ActivityAliases) == 0 && len(app.Services) == 0 &&
		len(app.Receivers) == 0 && len(app.Providers) == 0 && app.Debuggable == "" &&
		app.AllowBackup == "" && app.LocaleConfig == "" &&
		app.SupportsRtl == "" && app.ExtractNativeLibs == "" &&
		app.Theme == "" && app.Icon == "" && app.Label == "" &&
		app.UsesCleartextTraffic == "" && app.NetworkSecurityConfig == "" &&
		app.FullBackupContent == "" && app.DataExtractionRules == "" && len(app.MetaData) == 0 {
		return
	}
	cm.HasApplication = true
	if cm.AppLine == 0 {
		cm.AppLine = 1
	}
	cm.AppToolsNode = app.ToolsNode
	cm.LocaleConfig = app.LocaleConfig
	cm.Icon = app.Icon
	cm.HasBanner = app.Banner != ""
	cm.UsesCleartextTraffic = parseBoolPtr(app.UsesCleartextTraffic)
	cm.FullBackupContent = app.FullBackupContent
	cm.DataExtractionRules = app.DataExtractionRules
	cm.NetworkSecurityConfig = app.NetworkSecurityConfig
	cm.AllowBackup = parseBoolPtr(app.AllowBackup)
	cm.Debuggable = parseBoolPtr(app.Debuggable)
	cm.SupportsRtl = parseBoolPtr(app.SupportsRtl)
	cm.ExtractNativeLibs = parseBoolPtr(app.ExtractNativeLibs)
	convertActivityComponents(m, app, cm)
	for _, alias := range app.ActivityAliases {
		_, categories, _, _ := extractIntentFilterDetails(alias.IntentFilters)
		for _, c := range categories {
			if c == "android.intent.category.LAUNCHER" && alias.TargetActivity != "" {
				cm.LauncherAliasTargets = append(cm.LauncherAliasTargets, alias.TargetActivity)
				break
			}
		}
	}
	convertServiceComponents(m, app, cm)
	convertReceiverComponents(m, app, cm)
	convertProviderComponents(m, app, cm)
}

func convertActivityComponents(m *Manifest, app *Application, cm *ConvertedManifest) {
	activityNodes := childrenByTag(applicationNode(m.Root), "activity")
	for i, a := range app.Activities {
		if toolsNodeRemoves(a.ToolsNode) {
			continue
		}
		actions, categories, schemes, hosts := extractIntentFilterDetails(a.IntentFilters)
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
			IntentFilterDataHosts:   hosts,
			ExportedRaw:             a.Exported,
			HasBanner:               a.Banner != "",
			HasNavGraph:             len(a.NavGraphs) > 0,
			FirstFilterAction:       firstFilterAction(a.IntentFilters),
			FirstFilterCategory:     firstFilterCategory(a.IntentFilters),
		})
	}
	aliasNodes := childrenByTag(applicationNode(m.Root), "activity-alias")
	for i, a := range app.ActivityAliases {
		if toolsNodeRemoves(a.ToolsNode) {
			continue
		}
		actions, categories, schemes, hosts := extractIntentFilterDetails(a.IntentFilters)
		cm.Activities = append(cm.Activities, ConvertedComponent{
			Tag:                     "activity-alias",
			Name:                    a.Name,
			Line:                    lineForNodeAt(aliasNodes, i),
			Exported:                parseBoolPtr(a.Exported),
			Permission:              a.Permission,
			HasIntentFilter:         len(a.IntentFilters) > 0,
			ParentTag:               "application",
			IntentFilterActions:     actions,
			IntentFilterCategories:  categories,
			IntentFilterDataSchemes: schemes,
			IntentFilterDataHosts:   hosts,
			ExportedRaw:             a.Exported,
			HasBanner:               a.Banner != "",
			HasNavGraph:             len(a.NavGraphs) > 0,
			FirstFilterAction:       firstFilterAction(a.IntentFilters),
			FirstFilterCategory:     firstFilterCategory(a.IntentFilters),
		})
	}
}

// toolsNodeRemoves reports whether a tools:node value drops the element in the manifest merger.
func toolsNodeRemoves(node string) bool {
	return node == ToolsNodeRemove || node == ToolsNodeRemoveAll
}

func convertServiceComponents(m *Manifest, app *Application, cm *ConvertedManifest) {
	serviceNodes := childrenByTag(applicationNode(m.Root), "service")
	for i, s := range app.Services {
		if toolsNodeRemoves(s.ToolsNode) {
			continue
		}
		actions, categories, schemes, hosts := extractIntentFilterDetails(s.IntentFilters)
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
			IntentFilterDataHosts:   hosts,
			ExportedRaw:             s.Exported,
			HasNavGraph:             len(s.NavGraphs) > 0,
			FirstFilterAction:       firstFilterAction(s.IntentFilters),
			FirstFilterCategory:     firstFilterCategory(s.IntentFilters),
		})
	}
}

func convertReceiverComponents(m *Manifest, app *Application, cm *ConvertedManifest) {
	receiverNodes := childrenByTag(applicationNode(m.Root), "receiver")
	for i, r := range app.Receivers {
		if toolsNodeRemoves(r.ToolsNode) {
			continue
		}
		actions, categories, schemes, hosts := extractIntentFilterDetails(r.IntentFilters)
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
			IntentFilterDataHosts:   hosts,
			ExportedRaw:             r.Exported,
			HasNavGraph:             len(r.NavGraphs) > 0,
			FirstFilterAction:       firstFilterAction(r.IntentFilters),
			FirstFilterCategory:     firstFilterCategory(r.IntentFilters),
			MetaDataEntries:         metaEntries,
		})
	}
}

func convertProviderComponents(m *Manifest, app *Application, cm *ConvertedManifest) {
	providerNodes := childrenByTag(applicationNode(m.Root), "provider")
	for i, p := range app.Providers {
		if toolsNodeRemoves(p.ToolsNode) {
			continue
		}
		// A provider guarded only by a read, write or path permission is still protected.
		permission := p.Permission
		for _, candidate := range []string{p.ReadPermission, p.WritePermission} {
			if permission == "" {
				permission = candidate
			}
		}
		if permission == "" && len(p.PathPermissions) > 0 {
			permission = "path-permission"
		}
		cm.Providers = append(cm.Providers, ConvertedComponent{
			Tag:        "provider",
			Name:       p.Name,
			Line:       lineForNodeAt(providerNodes, i),
			Exported:   parseBoolPtr(p.Exported),
			Permission: permission,
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
// data schemes and hosts from a slice of IntentFilters.
func extractIntentFilterDetails(filters []IntentFilter) (actions, categories, schemes, hosts []string) {
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
			if d.Host != "" {
				hosts = append(hosts, d.Host)
			}
		}
	}
	return
}

// firstFilterAction is the name of the first <action> of the first <intent-filter>.
func firstFilterAction(filters []IntentFilter) string {
	if len(filters) == 0 || len(filters[0].Actions) == 0 {
		return ""
	}
	return filters[0].Actions[0].Name
}

// firstFilterCategory is the name of the first <category> of the first <intent-filter>.
func firstFilterCategory(filters []IntentFilter) string {
	if len(filters) == 0 || len(filters[0].Categories) == 0 {
		return ""
	}
	return filters[0].Categories[0].Name
}
