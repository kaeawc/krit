package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of the androidx.work:work-runtime 2.12.0 checks that join source with AndroidManifest.xml. Both run in the
// manifest-source scope: once per project, with every parsed manifest and source file.

const (
	workInitializationProvider = "androidx.startup.InitializationProvider"
	workManagerInitializer     = "androidx.work.WorkManagerInitializer"
	workSystemForegroundSvc    = "androidx.work.impl.foreground.SystemForegroundService"
	workForegroundInfoClass    = "androidx.work.ForegroundInfo"
	workRemoveInitializerMsg   = "Remove androidx.work.WorkManagerInitializer from your AndroidManifest.xml when using on-demand initialization."
)

// RemoveWorkManagerInitializerRule mirrors RemoveWorkManagerInitializerDetector.
type RemoveWorkManagerInitializerRule struct {
	AndroidRule
}

// Confidence is high: the Application must implement Configuration.Provider and the manifests are read directly.
func (r *RemoveWorkManagerInitializerRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *RemoveWorkManagerInitializerRule) check(ctx *api.Context) {
	if !workMayBePresent(ctx) {
		return
	}
	files := workProjectFiles(ctx)
	p := buildWorkProject(files)
	roots := map[string]string{}
	providerModules := map[string]bool{}
	var order []string
	for _, c := range p.classes {
		if c.abstract || !p.inheritsFrom(c, workApplicationClass) || !p.inheritsFrom(c, workConfigProvider) {
			continue
		}
		// Lint honors a suppression on the Application class itself.
		if workSuppressedAt(c, r.RuleName, r.RuleSetName) {
			continue
		}
		root := workModuleRoot(c.file.Path, roots)
		if !providerModules[root] {
			providerModules[root] = true
			order = append(order, root)
		}
	}
	if len(order) == 0 {
		return
	}
	removed := false
	for _, m := range ctx.Manifests {
		if _, rm := workInitializerLocation(m); rm {
			removed = true
		}
	}
	if removed {
		return
	}
	for _, root := range order {
		var at *manifest.Node
		var path string
		for _, m := range ctx.Manifests {
			if workModuleRoot(m.Path, roots) != root {
				continue
			}
			if node, _ := workInitializerLocation(m); node != nil {
				at, path = node, m.Path
			}
		}
		if at != nil {
			ctx.Emit(r.Finding(&scanner.File{Path: path}, at.Line, at.Col, workRemoveInitializerMsg))
		}
	}
}

// workInitializerLocation walks a manifest's <application> elements as the detector's visitElement does: the
// location is the application, then the InitializationProvider, then the WorkManagerInitializer meta-data, and
// removed reports a tools:node="remove" on either.
func workInitializerLocation(m *manifest.Manifest) (at *manifest.Node, removed bool) {
	if m == nil || m.Root == nil {
		return nil, false
	}
	for _, app := range workDescendants(m.Root, "application") {
		if at == nil {
			at = app
		}
		if provider := workFirstNamed(app, "provider", workInitializationProvider); provider != nil {
			at = provider
			if provider.AttrValue("tools:node") == "remove" {
				removed = true
			}
		}
		if meta := workFirstNamed(app, "meta-data", workManagerInitializer); meta != nil && !removed {
			at = meta
			if meta.AttrValue("tools:node") == "remove" {
				removed = true
			}
		}
	}
	return at, removed
}

// workDescendants returns every element with the tag under n, in document order (getElementsByTagName).
func workDescendants(n *manifest.Node, tag string) []*manifest.Node {
	var out []*manifest.Node
	var walk func(*manifest.Node)
	walk = func(cur *manifest.Node) {
		for _, child := range cur.Children {
			if child.Tag == tag {
				out = append(out, child)
			}
			walk(child)
		}
	}
	walk(n)
	return out
}

func workFirstNamed(n *manifest.Node, tag, name string) *manifest.Node {
	for _, el := range workDescendants(n, tag) {
		if el.AttrValue("android:name") == name {
			return el
		}
	}
	return nil
}

// workSuppressedAt reports whether the class declaration carries a suppression of the rule.
func workSuppressedAt(c *workClass, ruleID, ruleSet string) bool {
	file := c.file
	filter := file.Suppression
	if filter == nil {
		filter = scanner.BuildSuppressionFilter(file, nil, allRuleExcludes(), "").WithRuleAliases(AllSuppressionAliases())
		file.Suppression = filter
		file.SuppressionIdx = filter.Annotations()
	}
	name := lintClassNameNode(file, c.idx)
	if name == 0 {
		name = c.idx
	}
	return filter.IsSuppressedAtColumn(ruleID, ruleSet, file.FlatRow(name)+1, file.FlatCol(name)+1)
}

// SpecifyForegroundServiceTypeRule mirrors SpecifyForegroundServiceTypeIssueDetector.
type SpecifyForegroundServiceTypeRule struct {
	AndroidRule
}

// Confidence is high: the type must fold to a constant and the manifests are read directly.
func (r *SpecifyForegroundServiceTypeRule) Confidence() float64 { return api.ConfidenceHigh }

// workServiceTypes are the detector's SERVICE_TYPE_MAPPING, in its iteration order.
var workServiceTypes = []struct {
	mask int64
	name string
}{
	{1, "dataSync"}, {2, "mediaPlayback"}, {4, "phoneCall"}, {8, "location"}, {16, "connectedDevice"},
	{32, "mediaProjection"},
}

// workServiceInfoConstants are the android.content.pm.ServiceInfo foreground service types.
var workServiceInfoConstants = map[string]int64{
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_MANIFEST":         -1,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_NONE":             0,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC":        1,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK":   2,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_PHONE_CALL":       4,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_LOCATION":         8,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_CONNECTED_DEVICE": 16,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PROJECTION": 32,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_CAMERA":           64,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_MICROPHONE":       128,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_HEALTH":           256,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_REMOTE_MESSAGING": 512,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_SYSTEM_EXEMPTED":  1024,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_SHORT_SERVICE":    2048,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PROCESSING": 8192,
	"android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE":      1073741824,
}

func (r *SpecifyForegroundServiceTypeRule) check(ctx *api.Context) {
	if !workMayBePresent(ctx) {
		return
	}
	known := map[string]bool{}
	for _, m := range ctx.Manifests {
		if m == nil || m.Root == nil {
			continue
		}
		for _, svc := range workDescendants(m.Root, "service") {
			if svc.AttrValue("android:name") != workSystemForegroundSvc {
				continue
			}
			if types, ok := svc.Attr("android:foregroundServiceType"); ok {
				for _, t := range strings.Split(types.Value, "|") {
					known[t] = true
				}
			}
		}
	}
	for _, file := range workProjectFiles(ctx) {
		if !strings.Contains(string(file.Content), "ForegroundInfo") {
			continue
		}
		for _, kind := range []string{"call_expression", "object_creation_expression"} {
			file.FlatWalkNodes(0, kind, func(idx uint32) {
				r.checkForegroundInfo(ctx, file, idx, known)
			})
		}
	}
}

func (r *SpecifyForegroundServiceTypeRule) checkForegroundInfo(ctx *api.Context, file *scanner.File, idx uint32, known map[string]bool) {
	call, ok := lintCallAt(file, idx)
	if !ok || call.Name != "ForegroundInfo" || len(call.Args) <= 2 || !lintTypeRefMatches(file, call.TypeText, workForegroundInfoClass) {
		return
	}
	value, ok := lintEvalInt(file, call.Arg(2, "foregroundServiceType"), workServiceInfoConstants)
	if !ok || value <= 0 {
		return
	}
	for _, t := range workServiceTypes {
		if value&t.mask > 0 && !known[t.name] {
			ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				"Missing "+t.name+" foregroundServiceType in the AndroidManifest.xml"))
		}
	}
}
