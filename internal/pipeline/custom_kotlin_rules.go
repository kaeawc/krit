package pipeline

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// capabilityNeeds* mirror the `Capability.NEEDS_*.name` values from
// tools/krit-rule-api. Kept as constants so a typo can't silently turn
// the plumbing into a no-op.
const (
	capabilityNeedsGradle      = "NEEDS_GRADLE"
	capabilityNeedsManifest    = "NEEDS_MANIFEST"
	capabilityNeedsResources   = "NEEDS_RESOURCES"
	capabilityNeedsModuleIndex = "NEEDS_MODULE_INDEX"
	capabilityNeedsCrossFile   = "NEEDS_CROSS_FILE"
)

func runKotlinPluginRulesAndMerge(ctx context.Context, args ProjectArgs, host ProjectHostState, indexResult IndexResult, crossFileResult *CrossFileResult, bundleHit bool) error {
	if len(args.CustomRuleJars) == 0 || bundleHit {
		return nil
	}
	daemon := indexResult.Daemon
	if daemon == nil {
		daemon = host.OracleDaemon
	}
	if daemon == nil {
		return fmt.Errorf("kotlin custom rules require the krit-types daemon; pass --daemon. Released krit binaries auto-download the matching krit-types.jar into ~/.krit/jars/ on first use; for dev builds run: cd tools/krit-types && ./gradlew shadowJar (or set KRIT_TYPES_JAR to an existing jar)")
	}

	list, err := daemon.ListPlugins(args.CustomRuleJars)
	if err != nil {
		return fmt.Errorf("list Kotlin custom rules: %w", err)
	}
	if err := reportPluginDiagnostics(host.Reporter, list.Diagnostics); err != nil {
		return err
	}
	ruleIDs, ruleOptions := selectPluginRules(list.Rules, args.Config)
	if len(ruleIDs) == 0 {
		return nil
	}

	var gradlePayload *oracle.PluginGradleProfile
	if anyRuleNeedsCapability(list.Rules, ruleIDs, capabilityNeedsGradle) && indexResult.LibraryFacts != nil {
		gradlePayload = buildGradlePayload(&indexResult.LibraryFacts.Profile)
	}
	var manifestPayload *oracle.PluginManifestProfile
	if anyRuleNeedsCapability(list.Rules, ruleIDs, capabilityNeedsManifest) {
		manifestPayload = buildManifestPayload(indexResult.AndroidProject)
	}
	var resourcesPayload *oracle.PluginResourcesProfile
	if anyRuleNeedsCapability(list.Rules, ruleIDs, capabilityNeedsResources) {
		resourcesPayload = buildResourcesPayload(indexResult.AndroidProject)
	}
	var modulesPayload *oracle.PluginModulesProfile
	if anyRuleNeedsCapability(list.Rules, ruleIDs, capabilityNeedsModuleIndex) {
		modulesPayload = buildModulesPayload(indexResult.Graph)
	}
	var crossFilePayload *oracle.PluginCrossFileProfile
	if anyRuleNeedsCapability(list.Rules, ruleIDs, capabilityNeedsCrossFile) {
		crossFilePayload = buildCrossFilePayload(indexResult.CodeIndex)
	}

	// Replace, don't append: on the delta / affected-set replay paths the prior
	// findings bundle is carried forward (ApplyDelta) and already holds the last
	// run's plugin findings. Drop these rules' prior rows before adding the
	// freshly regenerated set so the two copies don't double-count. A no-op on
	// the cold path, where no plugin findings are present yet.
	pluginRuleSet := make(map[string]bool, len(ruleIDs))
	for _, id := range ruleIDs {
		pluginRuleSet[id] = true
	}
	base := dropFindingsByRule(crossFileResult.Findings, pluginRuleSet)
	collector := scanner.NewFindingCollector(base.Len())
	collector.AppendColumns(&base)
	for _, file := range indexResult.KotlinFiles {
		if file == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		result, err := daemon.AnalyzePluginFile(args.CustomRuleJars, file.Path, file.Content, ruleIDs, ruleOptions, gradlePayload, manifestPayload, resourcesPayload, modulesPayload, crossFilePayload)
		if err != nil {
			return fmt.Errorf("run Kotlin custom rules on %s: %w", file.Path, err)
		}
		if len(result.Errors) > 0 {
			return fmt.Errorf("run Kotlin custom rules on %s: %s", file.Path, formatPluginErrors(result.Errors))
		}
		cols := pluginFindingsToColumns(result.Findings)
		cols = applySuppressionColumns(&cols, indexResult.SourceFiles())
		collector.AppendColumns(&cols)
	}
	crossFileResult.Findings = *collector.Columns()
	return nil
}

// anyRuleNeedsCapability reports whether any of the selected rules
// declared the named capability. Callers use this to skip building the
// matching wire payload when no rule will read it — keeps the wire
// size minimal for the common case where rules only need source.
func anyRuleNeedsCapability(loaded []oracle.PluginRuleDescriptor, selected []string, capability string) bool {
	if len(selected) == 0 {
		return false
	}
	wanted := make(map[string]struct{}, len(selected))
	for _, id := range selected {
		wanted[id] = struct{}{}
	}
	for _, rule := range loaded {
		if _, ok := wanted[rule.RuleID]; !ok {
			continue
		}
		for _, need := range rule.Needs {
			if need == capability {
				return true
			}
		}
	}
	return false
}

// buildGradlePayload projects a librarymodel.ProjectProfile into the
// narrow wire schema the krit-types daemon expects on the analyzeFile
// request. SDK ints use 0 as the "absent" sentinel (the krit-types
// parser converts to null), and deps are flat "group:name:version"
// strings — see PluginGradleProfile in internal/oracle/daemon.go.
func buildGradlePayload(profile *librarymodel.ProjectProfile) *oracle.PluginGradleProfile {
	if profile == nil {
		return nil
	}
	deps := make([]string, 0, len(profile.Dependencies))
	for _, dep := range profile.Dependencies {
		if dep.Group == "" || dep.Name == "" || dep.Version == "" {
			continue
		}
		deps = append(deps, dep.Group+":"+dep.Name+":"+dep.Version)
	}
	sort.Strings(deps)
	// Different configurations can re-list the same coord/version pair —
	// strip duplicates so the wire payload (and the Kotlin-side
	// `versionByCoord` map build) does the minimum work.
	deps = uniqueStrings(deps)
	return &oracle.PluginGradleProfile{
		MinSdk:            profile.MinSdkVersion,
		TargetSdk:         profile.TargetSdkVersion,
		CompileSdk:        profile.CompileSdkVersion,
		KotlinVersion:     profile.Kotlin.EffectiveCompilerVersion(),
		JavaTargetVersion: profile.JVM.EffectiveTargetBytecode(),
		AGPVersion:        profile.Android.EffectiveAGPVersion(),
		Deps:              deps,
	}
}

// uniqueStrings collapses consecutive duplicates in a sorted slice in
// place. Callers must sort first.
func uniqueStrings(in []string) []string {
	if len(in) < 2 {
		return in
	}
	w := 1
	for r := 1; r < len(in); r++ {
		if in[r] != in[r-1] {
			in[w] = in[r]
			w++
		}
	}
	return in[:w]
}

// buildManifestPayload parses the first AndroidManifest.xml the
// AndroidProject detector found and projects it into the narrow wire
// schema the krit-types daemon expects. Returns nil when no manifest
// path is known or the file cannot be parsed — the daemon then leaves
// `RuleContext.manifest` null for the rule.
func buildManifestPayload(project *android.Project) *oracle.PluginManifestProfile {
	if project == nil || len(project.ManifestPaths) == 0 {
		return nil
	}
	var manifest *android.Manifest
	for _, path := range project.ManifestPaths {
		parsed, err := android.ParseManifest(path)
		if err == nil && parsed != nil {
			manifest = parsed
			break
		}
	}
	if manifest == nil {
		return nil
	}
	out := &oracle.PluginManifestProfile{
		Package:   manifest.Package,
		MinSdk:    parseSdkInt(manifest.UsesSdk.MinSdkVersion),
		TargetSdk: parseSdkInt(manifest.UsesSdk.TargetSdkVersion),
	}
	for _, p := range manifest.UsesPermissions {
		if p.Name != "" {
			out.Permissions = append(out.Permissions, p.Name)
		}
	}
	for _, a := range manifest.Application.Activities {
		if a.Name == "" {
			continue
		}
		out.Activities = append(out.Activities, a.Name)
		if android.IsExported(android.Component{Name: a.Name, Exported: a.Exported, Permission: a.Permission}, len(a.IntentFilters) > 0) {
			out.ExportedActivities = append(out.ExportedActivities, a.Name)
		}
	}
	for _, s := range manifest.Application.Services {
		if s.Name == "" {
			continue
		}
		out.Services = append(out.Services, s.Name)
		if android.IsExported(android.Component{Name: s.Name, Exported: s.Exported, Permission: s.Permission}, len(s.IntentFilters) > 0) {
			out.ExportedServices = append(out.ExportedServices, s.Name)
		}
	}
	for _, r := range manifest.Application.Receivers {
		if r.Name == "" {
			continue
		}
		out.Receivers = append(out.Receivers, r.Name)
		if android.IsExported(android.Component{Name: r.Name, Exported: r.Exported, Permission: r.Permission}, len(r.IntentFilters) > 0) {
			out.ExportedReceivers = append(out.ExportedReceivers, r.Name)
		}
	}
	return out
}

func parseSdkInt(raw string) int {
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

// buildResourcesPayload scans every res/ directory the AndroidProject
// detector found and projects the merged ResourceIndex into the wire
// schema. Strings/colors/dimensions are encoded as `"name=value"`
// strings so the daemon-side parser can lean on `extractJsonStringArray`.
// Returns nil when no res/ dir is known or scanning fails entirely.
func buildResourcesPayload(project *android.Project) *oracle.PluginResourcesProfile {
	if project == nil || len(project.ResDirs) == 0 {
		return nil
	}
	merged := &android.ResourceIndex{}
	for _, dir := range project.ResDirs {
		idx, err := android.ScanResourceDir(dir)
		if err != nil || idx == nil {
			continue
		}
		merged = android.MergeResourceIndexes(merged, idx)
	}
	out := &oracle.PluginResourcesProfile{}
	for name, value := range merged.Strings {
		out.Strings = append(out.Strings, name+"="+value)
	}
	for name, value := range merged.Colors {
		out.Colors = append(out.Colors, name+"="+value)
	}
	for name, value := range merged.Dimensions {
		out.Dimensions = append(out.Dimensions, name+"="+value)
	}
	for name := range merged.Layouts {
		out.Layouts = append(out.Layouts, name)
	}
	for name := range merged.IDs {
		out.IDs = append(out.IDs, name)
	}
	out.Drawables = append(out.Drawables, merged.Drawables...)
	sort.Strings(out.Strings)
	sort.Strings(out.Drawables)
	sort.Strings(out.Layouts)
	sort.Strings(out.Colors)
	sort.Strings(out.Dimensions)
	sort.Strings(out.IDs)
	// Drawables is the only field built from a slice (across multi-res/
	// configurations), so only it can carry true duplicates after merge;
	// the map-sourced fields are unique by construction.
	out.Drawables = uniqueStrings(out.Drawables)
	if len(out.Strings) == 0 && len(out.Drawables) == 0 && len(out.Layouts) == 0 &&
		len(out.Colors) == 0 && len(out.Dimensions) == 0 && len(out.IDs) == 0 {
		return nil
	}
	return out
}

// buildModulesPayload encodes each Gradle module as a single
// pipe-delimited `"path|directory|dependsOn,..|sourceRoots,..."` string.
// Gradle module paths never contain pipes or commas, so the encoding
// stays unambiguous without escaping.
func buildModulesPayload(graph *module.Graph) *oracle.PluginModulesProfile {
	if graph == nil || len(graph.Modules) == 0 {
		return nil
	}
	paths := make([]string, 0, len(graph.Modules))
	for path := range graph.Modules {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := &oracle.PluginModulesProfile{}
	for _, path := range paths {
		mod := graph.Modules[path]
		if mod == nil {
			continue
		}
		deps := make([]string, 0, len(mod.Dependencies))
		for _, dep := range mod.Dependencies {
			if dep.ModulePath != "" {
				deps = append(deps, dep.ModulePath)
			}
		}
		out.Modules = append(out.Modules, strings.Join([]string{
			mod.Path,
			mod.Dir,
			strings.Join(deps, ","),
			strings.Join(mod.SourceRoots, ","),
		}, "|"))
	}
	if len(out.Modules) == 0 {
		return nil
	}
	return out
}

// buildCrossFilePayload projects the scanner.CodeIndex into the wire
// schema: declarations as `"fqn|kind|file|line|visibility"` and
// references pre-grouped by name as `"name|file1,file2,..."` (only
// non-comment references count, mirroring the dead-code use case the
// index is most often used for).
func buildCrossFilePayload(index *scanner.CodeIndex) *oracle.PluginCrossFileProfile {
	if index == nil {
		return nil
	}
	out := &oracle.PluginCrossFileProfile{}
	if len(index.Symbols) > 0 {
		out.Declarations = make([]string, 0, len(index.Symbols))
		for _, sym := range index.Symbols {
			if sym.FQN == "" {
				continue
			}
			out.Declarations = append(out.Declarations, strings.Join([]string{
				sym.FQN,
				sym.Kind,
				sym.File,
				strconv.Itoa(sym.Line),
				sym.Visibility,
			}, "|"))
		}
		sort.Strings(out.Declarations)
	}
	if len(index.References) > 0 {
		filesByName := map[string][]string{}
		seenPerName := map[string]map[string]struct{}{}
		for _, ref := range index.References {
			if ref.Name == "" || ref.InComment {
				continue
			}
			if seenPerName[ref.Name] == nil {
				seenPerName[ref.Name] = map[string]struct{}{}
			}
			if _, ok := seenPerName[ref.Name][ref.File]; ok {
				continue
			}
			seenPerName[ref.Name][ref.File] = struct{}{}
			filesByName[ref.Name] = append(filesByName[ref.Name], ref.File)
		}
		names := make([]string, 0, len(filesByName))
		for name := range filesByName {
			names = append(names, name)
		}
		sort.Strings(names)
		out.NonCommentRefsByName = make([]string, 0, len(names))
		for _, name := range names {
			files := filesByName[name]
			sort.Strings(files)
			out.NonCommentRefsByName = append(out.NonCommentRefsByName, name+"|"+strings.Join(files, ","))
		}
	}
	if len(out.Declarations) == 0 && len(out.NonCommentRefsByName) == 0 {
		return nil
	}
	return out
}

// selectPluginRules picks which jar-loaded rules to dispatch and collects
// each rule's configured options from `pluginRules:` in krit.yml. A rule
// is skipped when the user explicitly set `active: false`; the absence of
// any pluginRules entry preserves the daemon's default activation.
func selectPluginRules(loaded []oracle.PluginRuleDescriptor, cfg *config.Config) ([]string, map[string]map[string]interface{}) {
	ruleIDs := make([]string, 0, len(loaded))
	options := map[string]map[string]interface{}{}
	for _, rule := range loaded {
		if rule.RuleID == "" {
			continue
		}
		if active := cfg.IsPluginRuleActive(rule.RuleID); active != nil && !*active {
			continue
		}
		ruleIDs = append(ruleIDs, rule.RuleID)
		if opts := cfg.PluginRuleOptions(rule.RuleID); len(opts) > 0 {
			options[rule.RuleID] = opts
		}
	}
	return ruleIDs, options
}

func pluginFindingsToColumns(findings []oracle.PluginFinding) scanner.FindingColumns {
	if len(findings) == 0 {
		return scanner.FindingColumns{}
	}
	out := make([]scanner.Finding, 0, len(findings))
	for _, finding := range findings {
		out = append(out, scanner.Finding{
			File:       finding.File,
			Line:       finding.Line,
			Col:        finding.Column,
			StartByte:  finding.StartByte,
			EndByte:    finding.EndByte,
			RuleSet:    finding.RuleSet,
			Rule:       finding.RuleID,
			Severity:   finding.Severity,
			Message:    finding.Message,
			Confidence: finding.Confidence,
			Fix:        pluginFixToScanner(finding.Fix),
		})
	}
	return scanner.CollectFindings(out)
}

func pluginFixToScanner(fix *oracle.PluginFix) *scanner.Fix {
	if fix == nil {
		return nil
	}
	out := &scanner.Fix{
		StartLine:   fix.StartLine,
		EndLine:     fix.EndLine,
		Replacement: fix.Replacement,
	}
	if lvl, ok := rules.ParseFixLevel(fix.Safety); ok {
		out.Safety = uint8(lvl)
	} else {
		// Unknown/missing → semantic so --fix-level still gates it.
		out.Safety = uint8(rules.FixSemantic)
	}
	return out
}

// reportPluginDiagnostics surfaces per-jar load-time verdicts from the
// daemon (SDK-compat verdicts plus capability gate from
// docs/external-rules.md#capability-semantics). Errors fail the run
// because the daemon already refused to load any rules from those jars
// — silently dropping them would hide the fact that the rules never
// ran.
func reportPluginDiagnostics(reporter *diag.Reporter, diagnostics []oracle.PluginLoadDiagnostic) error {
	var fatal []string
	for _, d := range diagnostics {
		line := d.Format()
		if d.Level == oracle.PluginDiagError {
			fatal = append(fatal, line)
			continue
		}
		reporter.Warnf("%s\n", line)
	}
	if len(fatal) == 0 {
		return nil
	}
	sort.Strings(fatal)
	return fmt.Errorf(
		"custom rule jar(s) failed to load; see per-jar message for the fix (rebuild against the daemon's krit-rule-api version, or remove unsupported capability declarations):\n  %s",
		strings.Join(fatal, "\n  "),
	)
}

func formatPluginErrors(errors map[string]string) string {
	parts := make([]string, 0, len(errors))
	for ruleID, message := range errors {
		parts = append(parts, ruleID+": "+message)
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}
