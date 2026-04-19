package pipeline

// AndroidPhase runs project-level Android analysis: manifest, resource,
// Gradle, and icon rules. The phase owns the async resource/icon scan
// providers, dispatch wiring, and perf tracking. All three Android rule
// families dispatch through the unified rules.Dispatcher.

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// AndroidInput is the entry value for the Android phase.
type AndroidInput struct {
	// Project is the detected Android project (manifest paths, res dirs,
	// gradle paths). When nil or empty, Run returns an empty result
	// without doing any work.
	Project *android.AndroidProject
	// ActiveRules is used to derive the set of active rule names (for
	// icon rule dispatch) and resource dependency mask.
	ActiveRules []*v2.Rule
	// Dispatcher routes manifest/resource/gradle rule execution. When
	// nil, Run returns no findings (but still walks the project for
	// parity with the pre-refactor empty-dispatcher behavior).
	Dispatcher *rules.Dispatcher
	// Providers is an optional async scan provider bundle. When nil,
	// Run constructs one lazily using runtime.NumCPU() workers.
	Providers *AndroidProjectProviders
	// Tracker, when non-nil, receives manifestAnalysis / resourceAnalysis
	// / gradleAnalysis child Trackers.
	Tracker perf.Tracker
}

// AndroidResult is the output of the Android phase.
type AndroidResult struct {
	// Findings holds every finding produced during the phase, already
	// stamped through the dispatcher's rule/finding post-processing.
	Findings scanner.FindingColumns
}

// AndroidPhase is the Phase wrapper for project-level Android analysis.
type AndroidPhase struct{}

// Name implements Phase.
func (AndroidPhase) Name() string { return "android" }

// Run implements Phase.
func (p AndroidPhase) Run(ctx context.Context, in AndroidInput) (AndroidResult, error) {
	if err := ctx.Err(); err != nil {
		return AndroidResult{}, err
	}
	if in.Project == nil || in.Project.IsEmpty() {
		return AndroidResult{}, nil
	}

	activeNames := make(map[string]bool, len(in.ActiveRules))
	collector := scanner.NewFindingCollector(len(in.Project.ManifestPaths)*4 + len(in.Project.ResDirs)*8 + len(in.Project.GradlePaths)*4)

	for _, rule := range in.ActiveRules {
		if rule != nil {
			activeNames[rule.ID] = true
		}
	}
	var resourceDeps rules.AndroidDataDependency
	for _, rule := range in.ActiveRules {
		if rule == nil {
			continue
		}
		dep := rules.AndroidDataDependency(rule.AndroidDeps)
		if dep&(rules.AndroidDepValues|rules.AndroidDepLayout|rules.AndroidDepResources|rules.AndroidDepValuesStrings|rules.AndroidDepValuesPlurals|rules.AndroidDepValuesArrays|rules.AndroidDepValuesExtraText) != 0 {
			resourceDeps |= dep
		}
	}
	valueKinds := androidValuesScanKinds(resourceDeps)
	providers := in.Providers
	if providers == nil {
		providers = NewAndroidProjectProviders(in.Project, CollectAndroidDependenciesV2(in.ActiveRules), runtime.NumCPU())
	}

	tracker := in.Tracker
	if tracker == nil {
		tracker = perf.New(false)
	}

	manifestTracker := tracker.Serial("manifestAnalysis")
	var (
		manifestParseDur time.Duration
		manifestRuleDur  time.Duration
	)
	for _, path := range in.Project.ManifestPaths {
		start := time.Now()
		parsed, err := android.ParseManifest(path)
		manifestParseDur += time.Since(start)
		if err != nil {
			continue
		}
		manifest := ConvertManifestForRules(android.ConvertManifest(parsed, path))
		start = time.Now()
		if in.Dispatcher != nil {
			// Manifest rules consume ctx.Manifest, not file.Content/Lines,
			// so skip re-reading the file we just parsed.
			file := &scanner.File{
				Path:     path,
				Language: scanner.LangXML,
				Metadata: manifest,
			}
			collector.AppendAll(in.Dispatcher.RunManifest(file, manifest))
		}
		manifestRuleDur += time.Since(start)
	}
	perf.AddEntry(manifestTracker, "manifestParse", manifestParseDur)
	perf.AddEntry(manifestTracker, "manifestRuleChecks", manifestRuleDur)
	manifestTracker.End()

	resourceTracker := tracker.Serial("resourceAnalysis")
	var (
		resourceScanDur         time.Duration
		resourceWaitDur         time.Duration
		resourceLayoutScanDur   time.Duration
		resourceValuesScanDur   time.Duration
		resourceDrawableScanDur time.Duration
		resourceValuesReadDur   time.Duration
		resourceValuesParseDur  time.Duration
		resourceValuesIndexDur  time.Duration
		resourceMergeDur        time.Duration
		resourceMaxLayoutDur    time.Duration
		resourceMaxValuesDur    time.Duration
		resourceMaxDrawableDur  time.Duration
		resourceRulesDur        time.Duration
		iconScanDur             time.Duration
		iconWaitDur             time.Duration
		iconRulesDur            time.Duration
	)
	for _, resDir := range in.Project.ResDirs {
		var (
			partialIndexes []*android.ResourceIndex
			hadResourceErr bool
		)
		if resourceDeps&rules.AndroidDepLayout != 0 {
			start := time.Now()
			var (
				idx   *android.ResourceIndex
				stats android.ResourceScanStats
				err   error
			)
			if future := providers.layout(resDir); future != nil {
				idx, stats, err = future.Await()
				resourceScanDur += future.Duration()
			} else {
				idx, stats, err = android.ScanLayoutResourcesWithStatsWorkers(resDir, runtime.NumCPU())
				resourceScanDur += time.Since(start)
			}
			resourceWaitDur += time.Since(start)
			if err == nil {
				partialIndexes = append(partialIndexes, idx)
				resourceLayoutScanDur += time.Duration(stats.LayoutScanMs) * time.Millisecond
				resourceDrawableScanDur += time.Duration(stats.DrawableScanMs) * time.Millisecond
				resourceMergeDur += time.Duration(stats.MergeMs) * time.Millisecond
				if d := time.Duration(stats.MaxLayoutScanMs) * time.Millisecond; d > resourceMaxLayoutDur {
					resourceMaxLayoutDur = d
				}
				if d := time.Duration(stats.MaxDrawableScanMs) * time.Millisecond; d > resourceMaxDrawableDur {
					resourceMaxDrawableDur = d
				}
			} else {
				hadResourceErr = true
			}
		}
		if resourceDeps&rules.AndroidDepValues != 0 {
			start := time.Now()
			var (
				idx   *android.ResourceIndex
				stats android.ResourceScanStats
				err   error
			)
			if future := providers.values(resDir); future != nil {
				idx, stats, err = future.Await()
				resourceScanDur += future.Duration()
			} else {
				idx, stats, err = android.ScanValuesResourcesWithStatsKindsWorkers(resDir, runtime.NumCPU(), valueKinds)
				resourceScanDur += time.Since(start)
			}
			resourceWaitDur += time.Since(start)
			if err == nil {
				partialIndexes = append(partialIndexes, idx)
				resourceValuesScanDur += time.Duration(stats.ValuesScanMs) * time.Millisecond
				resourceValuesReadDur += time.Duration(stats.ValuesReadMs) * time.Millisecond
				resourceValuesParseDur += time.Duration(stats.ValuesParseMs) * time.Millisecond
				resourceValuesIndexDur += time.Duration(stats.ValuesIndexMs) * time.Millisecond
				resourceMergeDur += time.Duration(stats.MergeMs) * time.Millisecond
				if d := time.Duration(stats.MaxValuesScanMs) * time.Millisecond; d > resourceMaxValuesDur {
					resourceMaxValuesDur = d
				}
			} else {
				hadResourceErr = true
			}
		}
		if !hadResourceErr && len(partialIndexes) > 0 {
			start := time.Now()
			mergedIdx := android.MergeResourceIndexes(partialIndexes...)
			if in.Dispatcher != nil {
				file := &scanner.File{
					Path:     resDir,
					Language: scanner.LangXML,
					Metadata: mergedIdx,
				}
				collector.AppendAll(in.Dispatcher.RunResource(file, mergedIdx))
			}
			resourceRulesDur += time.Since(start)
		}

		start := time.Now()
		var err error
		var iconIdx *android.IconIndex
		if future := providers.icon(resDir); future != nil {
			iconIdx, err = future.Await()
			iconScanDur += future.Duration()
		} else {
			iconIdx, err = android.ScanIconDirs(resDir)
			iconScanDur += time.Since(start)
		}
		iconWaitDur += time.Since(start)
		if err != nil {
			continue
		}
		start = time.Now()
		iconColumns := runActiveIconChecksColumns(iconIdx, activeNames)
		collector.AppendColumns(&iconColumns)
		iconRulesDur += time.Since(start)
	}
	perf.AddEntry(resourceTracker, "resourceDirScan", resourceWaitDur)
	perf.AddEntry(resourceTracker, "resourceDirScanCPU", resourceScanDur)
	perf.AddEntry(resourceTracker, "layoutDirScan", resourceLayoutScanDur)
	perf.AddEntry(resourceTracker, "valuesDirScanCPU", resourceValuesScanDur)
	perf.AddEntry(resourceTracker, "valuesFileRead", resourceValuesReadDur)
	perf.AddEntry(resourceTracker, "valuesXMLParseCPU", resourceValuesParseDur)
	perf.AddEntry(resourceTracker, "valuesIndexBuild", resourceValuesIndexDur)
	perf.AddEntry(resourceTracker, "resourceMerge", resourceMergeDur)
	perf.AddEntry(resourceTracker, "maxLayoutDirScan", resourceMaxLayoutDur)
	perf.AddEntry(resourceTracker, "maxValuesDirScan", resourceMaxValuesDur)
	perf.AddEntry(resourceTracker, "maxDrawableDirScan", resourceMaxDrawableDur)
	perf.AddEntry(resourceTracker, "drawableDirScan", resourceDrawableScanDur)
	perf.AddEntry(resourceTracker, "resourceRuleChecks", resourceRulesDur)
	perf.AddEntry(resourceTracker, "iconScan", iconWaitDur)
	perf.AddEntry(resourceTracker, "iconScanCPU", iconScanDur)
	perf.AddEntry(resourceTracker, "iconRuleChecks", iconRulesDur)
	resourceTracker.End()

	gradleTracker := tracker.Serial("gradleAnalysis")
	var (
		gradleReadParseDur time.Duration
		gradleRulesDur     time.Duration
	)
	for _, path := range in.Project.GradlePaths {
		start := time.Now()
		content, err := os.ReadFile(path)
		if err != nil {
			gradleReadParseDur += time.Since(start)
			continue
		}
		cfg, err := android.ParseBuildGradleContent(string(content))
		gradleReadParseDur += time.Since(start)
		if err != nil {
			continue
		}
		start = time.Now()
		if in.Dispatcher != nil {
			// Gradle rules consume ctx.GradleContent (populated by
			// RunGradle from file.Content); no rule reads file.Lines today.
			file := &scanner.File{
				Path:     path,
				Language: scanner.LangGradle,
				Content:  content,
				Metadata: cfg,
			}
			collector.AppendAll(in.Dispatcher.RunGradle(file, cfg))
		}
		gradleRulesDur += time.Since(start)
	}
	perf.AddEntry(gradleTracker, "gradleReadParse", gradleReadParseDur)
	perf.AddEntry(gradleTracker, "gradleRuleChecks", gradleRulesDur)
	gradleTracker.End()

	return AndroidResult{Findings: *collector.Columns()}, nil
}

// AndroidProjectProviders bundles the async scan futures for manifests,
// resources, and icons so IndexPhase can start them in parallel with
// Kotlin parsing and AndroidPhase can await them once the per-file pass
// is done.
type AndroidProjectProviders struct {
	project *android.AndroidProject
	deps    rules.AndroidDataDependency

	valuesFutures map[string]*android.ResourceScanFuture
	layoutFutures map[string]*android.ResourceScanFuture
	iconFutures   map[string]*android.IconScanFuture
}

// NewAndroidProjectProviders constructs a provider bundle for the given
// Android project and rule dependency mask. The bundle's futures are
// created (but not started) and sized via maxWorkers — call Start() to
// kick them off.
func NewAndroidProjectProviders(project *android.AndroidProject, deps rules.AndroidDataDependency, maxWorkers int) *AndroidProjectProviders {
	p := &AndroidProjectProviders{
		project:       project,
		deps:          deps,
		valuesFutures: make(map[string]*android.ResourceScanFuture),
		layoutFutures: make(map[string]*android.ResourceScanFuture),
		iconFutures:   make(map[string]*android.IconScanFuture),
	}
	if project == nil {
		return p
	}
	resourceWorkers := androidProviderWorkerCount(maxWorkers)
	iconWorkers := androidIconProviderWorkerCount(maxWorkers)
	resourceLimiter := make(chan struct{}, androidProviderStartConcurrency(maxWorkers))
	iconLimiter := make(chan struct{}, androidIconProviderStartConcurrency(maxWorkers))
	if deps&rules.AndroidDepValues != 0 {
		valueKinds := androidValuesScanKinds(deps)
		for _, resDir := range project.ResDirs {
			p.valuesFutures[resDir] = android.NewValuesScanFuture(resDir, resourceLimiter, valueKinds, resourceWorkers)
		}
	}
	if deps&rules.AndroidDepLayout != 0 {
		for _, resDir := range project.ResDirs {
			p.layoutFutures[resDir] = android.NewLayoutScanFuture(resDir, resourceLimiter, resourceWorkers)
		}
	}
	if deps&rules.AndroidDepIcons != 0 {
		for _, resDir := range project.ResDirs {
			p.iconFutures[resDir] = android.NewIconScanFuture(resDir, iconLimiter, iconWorkers)
		}
	}
	return p
}

// Start kicks off every registered scan future. Safe to call on a nil
// receiver.
func (p *AndroidProjectProviders) Start() {
	if p == nil {
		return
	}
	for _, future := range p.valuesFutures {
		future.Start()
	}
	for _, future := range p.layoutFutures {
		future.Start()
	}
	for _, future := range p.iconFutures {
		future.Start()
	}
}

func (p *AndroidProjectProviders) values(resDir string) *android.ResourceScanFuture {
	if p == nil {
		return nil
	}
	return p.valuesFutures[resDir]
}

func (p *AndroidProjectProviders) layout(resDir string) *android.ResourceScanFuture {
	if p == nil {
		return nil
	}
	return p.layoutFutures[resDir]
}

func (p *AndroidProjectProviders) icon(resDir string) *android.IconScanFuture {
	if p == nil {
		return nil
	}
	return p.iconFutures[resDir]
}

// CollectAndroidDependencies returns the union of every active rule's
// Android data dependency mask. Used to decide which resource/icon
// futures to spin up.
func CollectAndroidDependencies(activeRules []rules.Rule) rules.AndroidDataDependency {
	var deps rules.AndroidDataDependency
	for _, rule := range activeRules {
		deps |= rules.AndroidDependenciesOf(rule)
	}
	return deps
}

// CollectAndroidDependenciesV2 is the v2-native equivalent of
// CollectAndroidDependencies. It reads AndroidDeps directly from
// v2.Rule.AndroidDeps, falling back to the v1 wrapper for rules that
// set the field at registration time.
func CollectAndroidDependenciesV2(activeRules []*v2.Rule) rules.AndroidDataDependency {
	var deps rules.AndroidDataDependency
	for _, r := range activeRules {
		if r == nil {
			continue
		}
		deps |= rules.AndroidDataDependency(r.AndroidDeps)
	}
	return deps
}

func androidProviderWorkerCount(maxWorkers int) int {
	if maxWorkers < 1 {
		return 1
	}
	workers := maxWorkers / 4
	if workers < 2 && maxWorkers >= 8 {
		workers = 2
	}
	if workers < 1 {
		workers = 1
	}
	if workers > 4 {
		workers = 4
	}
	return workers
}

func androidIconProviderWorkerCount(maxWorkers int) int {
	if maxWorkers >= 16 {
		return 2
	}
	return 1
}

func androidProviderStartConcurrency(maxWorkers int) int {
	if maxWorkers >= 8 {
		return 2
	}
	return 1
}

func androidIconProviderStartConcurrency(maxWorkers int) int {
	return 1
}

func androidValuesScanKinds(_ rules.AndroidDataDependency) android.ValuesScanKind {
	return android.ValuesScanAll
}

// ConvertManifestForRules adapts an android.ConvertedManifest (produced
// by the XML parser) into a rules.Manifest suitable for manifest rules.
func ConvertManifestForRules(m *android.ConvertedManifest) *rules.Manifest {
	rm := &rules.Manifest{
		Path:            m.Path,
		Package:         m.Package,
		MinSDK:          m.MinSDK,
		TargetSDK:       m.TargetSDK,
		UsesPermissions: append([]string(nil), m.UsesPermissions...),
		Permissions:     append([]string(nil), m.Permissions...),
	}
	for _, f := range m.UsesFeatures {
		rm.UsesFeatures = append(rm.UsesFeatures, rules.ManifestUsesFeature{
			Name:     f.Name,
			Required: f.Required,
			Line:     f.Line,
		})
	}

	for _, elem := range m.Elements {
		rm.Elements = append(rm.Elements, rules.ManifestElement{
			Tag:       elem.Tag,
			Line:      elem.Line,
			ParentTag: elem.ParentTag,
		})
	}
	if m.HasUsesSdk {
		rm.UsesSdk = &rules.ManifestElement{
			Tag:       "uses-sdk",
			Line:      m.UsesSdkLine,
			ParentTag: "manifest",
		}
	}

	if m.HasApplication {
		app := &rules.ManifestApplication{
			Line:                  m.AppLine,
			AllowBackup:           m.AllowBackup,
			Debuggable:            m.Debuggable,
			LocaleConfig:          m.LocaleConfig,
			SupportsRtl:           m.SupportsRtl,
			ExtractNativeLibs:     m.ExtractNativeLibs,
			Icon:                  m.Icon,
			UsesCleartextTraffic:  m.UsesCleartextTraffic,
			FullBackupContent:     m.FullBackupContent,
			DataExtractionRules:   m.DataExtractionRules,
			NetworkSecurityConfig: m.NetworkSecurityConfig,
		}
		for _, activity := range m.Activities {
			app.Activities = append(app.Activities, rules.ManifestComponent{
				Tag:                     activity.Tag,
				Name:                    activity.Name,
				Line:                    activity.Line,
				Exported:                activity.Exported,
				Permission:              activity.Permission,
				HasIntentFilter:         activity.HasIntentFilter,
				ParentTag:               activity.ParentTag,
				IntentFilterActions:     activity.IntentFilterActions,
				IntentFilterCategories:  activity.IntentFilterCategories,
				IntentFilterDataSchemes: activity.IntentFilterDataSchemes,
			})
		}
		for _, service := range m.Services {
			app.Services = append(app.Services, rules.ManifestComponent{
				Tag:                     service.Tag,
				Name:                    service.Name,
				Line:                    service.Line,
				Exported:                service.Exported,
				Permission:              service.Permission,
				HasIntentFilter:         service.HasIntentFilter,
				ParentTag:               service.ParentTag,
				IntentFilterActions:     service.IntentFilterActions,
				IntentFilterCategories:  service.IntentFilterCategories,
				IntentFilterDataSchemes: service.IntentFilterDataSchemes,
			})
		}
		for _, receiver := range m.Receivers {
			var metaEntries []rules.ManifestMetaData
			for _, md := range receiver.MetaDataEntries {
				metaEntries = append(metaEntries, rules.ManifestMetaData{
					Name:     md.Name,
					Value:    md.Value,
					Resource: md.Resource,
				})
			}
			app.Receivers = append(app.Receivers, rules.ManifestComponent{
				Tag:                     receiver.Tag,
				Name:                    receiver.Name,
				Line:                    receiver.Line,
				Exported:                receiver.Exported,
				Permission:              receiver.Permission,
				HasIntentFilter:         receiver.HasIntentFilter,
				ParentTag:               receiver.ParentTag,
				IntentFilterActions:     receiver.IntentFilterActions,
				IntentFilterCategories:  receiver.IntentFilterCategories,
				IntentFilterDataSchemes: receiver.IntentFilterDataSchemes,
				MetaDataEntries:         metaEntries,
			})
		}
		for _, provider := range m.Providers {
			app.Providers = append(app.Providers, rules.ManifestComponent{
				Tag:        provider.Tag,
				Name:       provider.Name,
				Line:       provider.Line,
				Exported:   provider.Exported,
				Permission: provider.Permission,
				ParentTag:  provider.ParentTag,
			})
		}
		rm.Application = app
	}

	return rm
}

// RunActiveIconChecks runs all active icon rules and returns the
// aggregated findings. Exposed for callers that don't want the columns
// form.
func RunActiveIconChecks(idx *android.IconIndex, activeNames map[string]bool) []scanner.Finding {
	columns := runActiveIconChecksColumns(idx, activeNames)
	return columns.Findings()
}

func runActiveIconChecksColumns(idx *android.IconIndex, activeNames map[string]bool) scanner.FindingColumns {
	collector := scanner.NewFindingCollector(8)
	if activeNames["IconDensities"] {
		collector.AppendAll(rules.CheckIconDensities(idx))
	}
	if activeNames["IconDipSize"] {
		collector.AppendAll(rules.CheckIconDipSize(idx))
	}
	if activeNames["IconDuplicates"] {
		collector.AppendAll(rules.CheckIconDuplicates(idx))
	}
	if activeNames["GifUsage"] {
		collector.AppendAll(rules.CheckGifUsage(idx))
	}
	if activeNames["ConvertToWebp"] {
		collector.AppendAll(rules.CheckConvertToWebp(idx))
	}
	if activeNames["IconMissingDensityFolder"] {
		collector.AppendAll(rules.CheckIconMissingDensityFolder(idx))
	}
	if activeNames["IconExpectedSize"] {
		collector.AppendAll(rules.CheckIconExpectedSize(idx))
	}
	return *collector.Columns()
}

// RunActiveIconChecksColumns exposes the columns form of the active icon
// check runner for callers that want to merge findings columnar.
func RunActiveIconChecksColumns(idx *android.IconIndex, activeNames map[string]bool) scanner.FindingColumns {
	return runActiveIconChecksColumns(idx, activeNames)
}
