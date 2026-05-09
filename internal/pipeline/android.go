package pipeline

// AndroidPhase runs project-level Android analysis: manifest, resource,
// Gradle, and icon rules. The phase owns the async resource/icon scan
// providers, dispatch wiring, and perf tracking. All three Android rule
// families dispatch through the unified rules.Dispatcher.

import (
	"context"
	"encoding/hex"
	"os"
	"runtime"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// AndroidInput is the entry value for the Android phase.
type AndroidInput struct {
	// Project is the detected Android project (manifest paths, res dirs,
	// gradle paths). When nil or empty, Run returns an empty result
	// without doing any work.
	Project *android.Project
	// ActiveRules is used to derive the set of active rule names (for
	// icon rule dispatch) and resource dependency mask.
	ActiveRules []*api.Rule
	// Dispatcher routes manifest/resource/gradle rule execution. When
	// nil, Run returns no findings (but still walks the project for
	// parity with the pre-refactor empty-dispatcher behavior).
	Dispatcher *rules.Dispatcher
	// SourceFiles are parsed Kotlin files. Resource-backed source rules run
	// over these once the merged ResourceIndex is available.
	SourceFiles []*scanner.File
	// SourcePaths is the collected source set used to validate warm
	// resource-source bundle cache hits when parsing was skipped.
	SourcePaths []string
	// SourceHashes are content hashes for SourcePaths already validated by
	// the incremental findings cache. They let warm Android resource-source
	// bundle loads avoid rehashing every source file after parse skip.
	SourceHashes map[string]string
	// Providers is an optional async scan provider bundle. When nil,
	// Run constructs one lazily using runtime.NumCPU() workers.
	Providers *AndroidProjectProviders
	// Tracker, when non-nil, receives manifestAnalysis / resourceAnalysis
	// / gradleAnalysis child Trackers.
	Tracker perf.Tracker

	// RuleHash is the active-rule + config fingerprint produced by
	// cache.ComputeConfigHash. Reserved for the upcoming
	// android-findings-cache: any change in active rules or their config
	// must invalidate cached project-level findings, so we plumb it now
	// even though Run does not yet read it.
	RuleHash string
	// LibraryFactsFP is the fingerprint of the library-model facts
	// (Gradle, version-catalog, dependency-derived facts) that Android
	// rules consume. A change here means library-aware findings must be
	// recomputed. Plumbed for android-findings-cache.
	LibraryFactsFP string
	// JavaSemanticFactsFP is the fingerprint of the Java semantic facts
	// (call sites, class shells) that Android rules consume. Plumbed for
	// android-findings-cache.
	JavaSemanticFactsFP string

	// CacheDir is the android-findings-cache root for this run. When
	// empty, the phase falls back to the un-cached path.
	CacheDir string
	// CacheWriter persists fresh cache entries asynchronously. When nil,
	// the phase falls back to the un-cached path.
	CacheWriter *scanner.AndroidCacheWriter
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

// androidResourceDeps computes the AndroidDataDependency mask for the
// active rule set, restricted to resource-related dependency bits.
func (AndroidPhase) androidResourceDeps(activeRules []*api.Rule) rules.AndroidDataDependency {
	var resourceDeps rules.AndroidDataDependency
	for _, rule := range activeRules {
		if rule == nil {
			continue
		}
		dep := rules.AndroidDataDependency(rule.AndroidDeps)
		if dep&(rules.AndroidDepValues|rules.AndroidDepLayout|rules.AndroidDepResources|rules.AndroidDepValuesStrings|rules.AndroidDepValuesPlurals|rules.AndroidDepValuesArrays|rules.AndroidDepValuesExtraText) != 0 {
			resourceDeps |= dep
		}
	}
	return resourceDeps
}

type androidPhaseNeeds struct {
	manifest  bool
	resources bool
	icons     bool
	gradle    bool
}

func (n androidPhaseNeeds) any() bool {
	return n.manifest || n.resources || n.icons || n.gradle
}

func classifyAndroidPhaseNeeds(activeRules []*api.Rule) androidPhaseNeeds {
	var needs androidPhaseNeeds
	for _, rule := range activeRules {
		if rule == nil {
			continue
		}
		deps := rules.AndroidDataDependency(rule.AndroidDeps)
		if rule.Needs.Has(api.NeedsManifest) || deps&rules.AndroidDepManifest != 0 {
			needs.manifest = true
		}
		if rule.Needs.Has(api.NeedsResources) || deps&(rules.AndroidDepLayout|rules.AndroidDepValues) != 0 {
			needs.resources = true
		}
		if deps&rules.AndroidDepIcons != 0 {
			needs.icons = true
		}
		if rule.Needs.Has(api.NeedsGradle) || deps&rules.AndroidDepGradle != 0 {
			needs.gradle = true
		}
	}
	return needs
}

func hasResourceSourceRules(activeRules []*api.Rule) bool {
	for _, rule := range activeRules {
		if rule == nil || !rule.Needs.Has(api.NeedsResources) || len(rule.NodeTypes) == 0 {
			continue
		}
		for _, lang := range api.RuleLanguages(rule) {
			if lang != scanner.LangXML {
				return true
			}
		}
	}
	return false
}

// androidResourceTimings accumulates per-directory resource scan durations.
type androidResourceTimings struct {
	scanDur         time.Duration
	waitDur         time.Duration
	layoutScanDur   time.Duration
	valuesScanDur   time.Duration
	drawableScanDur time.Duration
	valuesReadDur   time.Duration
	valuesParseDur  time.Duration
	valuesIndexDur  time.Duration
	mergeDur        time.Duration
	maxLayoutDur    time.Duration
	maxValuesDur    time.Duration
	maxDrawableDur  time.Duration
	rulesDur        time.Duration
	sourceRulesDur  time.Duration
	iconScanDur     time.Duration
	iconWaitDur     time.Duration
	iconRulesDur    time.Duration
}

// scanLayoutDir scans the layout resources for one resDir and accumulates
// its timing/stat contributions into t. Returns the index and whether an
// error occurred.
func (AndroidPhase) scanLayoutDir(resDir string, providers *AndroidProjectProviders, t *androidResourceTimings) (*android.ResourceIndex, bool) {
	start := time.Now()
	var (
		idx   *android.ResourceIndex
		stats android.ResourceScanStats
		err   error
	)
	if future := providers.layout(resDir); future != nil {
		idx, stats, err = future.Await()
		t.scanDur += future.Duration()
	} else {
		idx, stats, err = android.ScanLayoutResourcesWithStatsWorkers(resDir, runtime.NumCPU())
		t.scanDur += time.Since(start)
	}
	t.waitDur += time.Since(start)
	if err != nil {
		return nil, true
	}
	t.layoutScanDur += time.Duration(stats.LayoutScanMs) * time.Millisecond
	t.drawableScanDur += time.Duration(stats.DrawableScanMs) * time.Millisecond
	t.mergeDur += time.Duration(stats.MergeMs) * time.Millisecond
	if d := time.Duration(stats.MaxLayoutScanMs) * time.Millisecond; d > t.maxLayoutDur {
		t.maxLayoutDur = d
	}
	if d := time.Duration(stats.MaxDrawableScanMs) * time.Millisecond; d > t.maxDrawableDur {
		t.maxDrawableDur = d
	}
	return idx, false
}

// scanValuesDir scans the values resources for one resDir and accumulates
// timing/stat contributions. Returns the index and whether an error occurred.
func (AndroidPhase) scanValuesDir(resDir string, providers *AndroidProjectProviders, valueKinds android.ValuesScanKind, t *androidResourceTimings) (*android.ResourceIndex, bool) {
	start := time.Now()
	var (
		idx   *android.ResourceIndex
		stats android.ResourceScanStats
		err   error
	)
	if future := providers.values(resDir); future != nil {
		idx, stats, err = future.Await()
		t.scanDur += future.Duration()
	} else {
		idx, stats, err = android.ScanValuesResourcesWithStatsKindsWorkers(resDir, runtime.NumCPU(), valueKinds)
		t.scanDur += time.Since(start)
	}
	t.waitDur += time.Since(start)
	if err != nil {
		return nil, true
	}
	t.valuesScanDur += time.Duration(stats.ValuesScanMs) * time.Millisecond
	t.valuesReadDur += time.Duration(stats.ValuesReadMs) * time.Millisecond
	t.valuesParseDur += time.Duration(stats.ValuesParseMs) * time.Millisecond
	t.valuesIndexDur += time.Duration(stats.ValuesIndexMs) * time.Millisecond
	t.mergeDur += time.Duration(stats.MergeMs) * time.Millisecond
	if d := time.Duration(stats.MaxValuesScanMs) * time.Millisecond; d > t.maxValuesDur {
		t.maxValuesDur = d
	}
	return idx, false
}

// scanIconDir scans the icon resources for one resDir and accumulates icon
// timing contributions. Returns the index and whether an error occurred.
func (AndroidPhase) scanIconDir(resDir string, providers *AndroidProjectProviders, t *androidResourceTimings) (*android.IconIndex, bool) {
	start := time.Now()
	var (
		iconIdx *android.IconIndex
		err     error
	)
	if future := providers.icon(resDir); future != nil {
		iconIdx, err = future.Await()
		t.iconScanDur += future.Duration()
	} else {
		iconIdx, err = android.ScanIconDirs(resDir)
		t.iconScanDur += time.Since(start)
	}
	t.iconWaitDur += time.Since(start)
	return iconIdx, err != nil
}

// resourceCacheStats counts cache outcomes during the resource loop,
// summed across all resDirs.
type resourceCacheStats struct {
	hits   int
	misses int
	skips  int
}

// runResourceDir processes one resource directory: scans layout/values as
// needed, merges the partial indexes, dispatches resource rules, and scans
// icons when requested. Returns the merged ResourceIndex (nil on error) for
// later source rule dispatch.
//
// The dispatcher's RunResource call (which dominates resourceAnalysis on
// large projects) is cached by resDir-content fingerprint. On a cache hit,
// scan + merge are skipped unless resource-source rules need the merged index
// to loop over SourceFiles after this returns.
func (p AndroidPhase) runResourceDir(in AndroidInput, resDir string, resDirFP string, resourceDeps rules.AndroidDataDependency, valueKinds android.ValuesScanKind, scanIcons bool, needResourceSource bool, providers *AndroidProjectProviders, collector *scanner.FindingCollector, resourceBundleCollector *scanner.FindingCollector, t *androidResourceTimings, stats *resourceCacheStats) *android.ResourceIndex {
	resourceCacheHit, resourceKey := p.loadCachedResourceFindings(in, resDir, resDirFP, resourceDeps, valueKinds, collector, resourceBundleCollector, stats)
	partialIndexes, hadResourceErr := p.scanResourceIndexes(resDir, resourceDeps, valueKinds, needResourceSource, resourceCacheHit, providers, t)
	mergedIdx := p.mergeResourceIndexes(in, resDir, resourceKey, resourceCacheHit, partialIndexes, hadResourceErr, stats, collector, resourceBundleCollector, t)
	p.scanResourceIcons(in, resDir, scanIcons, providers, collector, t)
	return mergedIdx
}

func (p AndroidPhase) loadCachedResourceFindings(in AndroidInput, resDir, resDirFP string, resourceDeps rules.AndroidDataDependency, valueKinds android.ValuesScanKind, collector *scanner.FindingCollector, resourceBundleCollector *scanner.FindingCollector, stats *resourceCacheStats) (resourceCacheHit bool, resourceKey string) {
	cacheable := in.CacheDir != "" && in.CacheWriter != nil && in.RuleHash != ""
	if !cacheable || stats == nil || resDirFP == "" || resourceDeps&(rules.AndroidDepLayout|rules.AndroidDepValues) == 0 {
		return false, ""
	}
	resourceKey = in.resourceKey(resDir, resDirFP, resourceDeps, valueKinds)
	cached, ok := scanner.LoadAndroidFindings(in.CacheDir, resourceKey)
	if !ok {
		stats.misses++
		return false, resourceKey
	}
	collector.AppendColumns(&cached)
	if resourceBundleCollector != nil {
		resourceBundleCollector.AppendColumns(&cached)
	}
	stats.hits++
	return true, resourceKey
}

func (p AndroidPhase) scanResourceIndexes(resDir string, resourceDeps rules.AndroidDataDependency, valueKinds android.ValuesScanKind, needResourceSource bool, resourceCacheHit bool, providers *AndroidProjectProviders, t *androidResourceTimings) (partialIndexes []*android.ResourceIndex, hadResourceErr bool) {
	if resourceCacheHit && !needResourceSource {
		return nil, false
	}
	if resourceDeps&rules.AndroidDepLayout != 0 {
		idx, hadErr := p.scanLayoutDir(resDir, providers, t)
		if hadErr {
			hadResourceErr = true
		} else {
			partialIndexes = append(partialIndexes, idx)
		}
	}
	if resourceDeps&rules.AndroidDepValues != 0 {
		idx, hadErr := p.scanValuesDir(resDir, providers, valueKinds, t)
		if hadErr {
			hadResourceErr = true
		} else {
			partialIndexes = append(partialIndexes, idx)
		}
	}
	return partialIndexes, hadResourceErr
}

func (p AndroidPhase) mergeResourceIndexes(in AndroidInput, resDir, resourceKey string, resourceCacheHit bool, partialIndexes []*android.ResourceIndex, hadResourceErr bool, stats *resourceCacheStats, collector *scanner.FindingCollector, resourceBundleCollector *scanner.FindingCollector, t *androidResourceTimings) *android.ResourceIndex {
	if hadResourceErr || len(partialIndexes) == 0 || in.Dispatcher == nil {
		return nil
	}
	mergedIdx := android.MergeResourceIndexes(partialIndexes...)
	if resourceCacheHit {
		return mergedIdx
	}

	start := time.Now()
	file := &scanner.File{
		Path:     resDir,
		Language: scanner.LangXML,
		Metadata: mergedIdx,
	}

	canCache := resourceKey != "" && stats != nil && in.CacheWriter != nil && in.CacheDir != "" && in.RuleHash != ""
	if canCache {
		cols := in.Dispatcher.RunResource(file, mergedIdx)
		collector.AppendColumns(&cols)
		if resourceBundleCollector != nil {
			resourceBundleCollector.AppendColumns(&cols)
		}
		in.CacheWriter.Save(in.CacheDir, resourceKey, cols)
	} else {
		cols := in.Dispatcher.RunResource(file, mergedIdx)
		collector.AppendColumns(&cols)
		if resourceBundleCollector != nil {
			resourceBundleCollector.AppendColumns(&cols)
		}
	}

	t.rulesDur += time.Since(start)
	return mergedIdx
}

func (p AndroidPhase) scanResourceIcons(in AndroidInput, resDir string, scanIcons bool, providers *AndroidProjectProviders, collector *scanner.FindingCollector, t *androidResourceTimings) {
	if !scanIcons || in.Dispatcher == nil {
		return
	}
	cacheable := in.CacheDir != "" && in.CacheWriter != nil && in.RuleHash != ""
	var key string
	if cacheable {
		resDirFP := resDirContentFingerprint(resDir)
		key = in.iconKey(resDir, resDirFP)
		if cached, ok := scanner.LoadAndroidFindings(in.CacheDir, key); ok {
			collector.AppendColumns(&cached)
			return
		}
	}
	iconIdx, hadErr := p.scanIconDir(resDir, providers, t)
	if hadErr {
		return
	}
	start := time.Now()
	file := &scanner.File{
		Path:     resDir,
		Language: scanner.LangXML,
		Metadata: iconIdx,
	}
	iconColumns := in.Dispatcher.RunIcons(file, iconIdx)
	collector.AppendColumns(&iconColumns)
	if key != "" {
		in.CacheWriter.Save(in.CacheDir, key, iconColumns)
	}
	t.iconRulesDur += time.Since(start)
}

// manifestCacheStats counts cache outcomes during the manifest loop.
type manifestCacheStats struct {
	hits   int
	misses int
	skips  int
}

// manifestBundleFingerprint hashes the (path, content-hash) pairs of every
// manifest in the project. This lets a single manifest's cache key invalidate
// when *any* manifest changes — conservative behavior for merge-aware rules
// that can't be told which manifests overlay which without doing the merge.
func (AndroidPhase) manifestBundleFingerprint(paths []string) string {
	h := hashutil.Hasher().New()
	memo := hashutil.Default()
	for _, p := range paths {
		hx, err := memo.HashFile(p, nil)
		if err != nil {
			// Conservative miss-marker: include the path with an
			// "err" sentinel so the bundle fingerprint reflects
			// the failure and entries computed under it don't
			// silently match a later successful read.
			h.Write([]byte(p))
			h.Write([]byte("\x00err\x00"))
			continue
		}
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write([]byte(hx))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// runManifestPhase iterates over all manifest paths, parses each (or
// loads cached findings), dispatches manifest rules on miss, and queues
// fresh entries on the writer. Returns accumulated timing durations and
// cache outcome counts.
func (p AndroidPhase) runManifestPhase(in AndroidInput, collector *scanner.FindingCollector) (parseDur, ruleDur time.Duration, stats manifestCacheStats) {
	cacheable := in.CacheDir != "" && in.CacheWriter != nil && in.RuleHash != ""
	var bundleFP string
	var bundleKey string
	var bundleCollector *scanner.FindingCollector
	if cacheable {
		bundleFP = p.manifestBundleFingerprint(in.Project.ManifestPaths)
		bundleKey = in.manifestBundleKey(bundleFP)
		if cached, ok := scanner.LoadAndroidFindings(in.CacheDir, bundleKey); ok {
			collector.AppendColumns(&cached)
			stats.hits = len(in.Project.ManifestPaths)
			return 0, 0, stats
		}
		bundleCollector = scanner.NewFindingCollector(len(in.Project.ManifestPaths) * 4)
	}

	memo := hashutil.Default()
	for _, path := range in.Project.ManifestPaths {
		if cacheable {
			contentHash, err := memo.HashFile(path, nil)
			if err != nil {
				stats.skips++
			} else {
				key := in.manifestKey(contentHash, bundleFP)
				if cached, ok := scanner.LoadAndroidFindings(in.CacheDir, key); ok {
					collector.AppendColumns(&cached)
					bundleCollector.AppendColumns(&cached)
					stats.hits++
					continue
				}
				stats.misses++
				cols, pdur, rdur := p.runManifestOne(in, path)
				parseDur += pdur
				ruleDur += rdur
				collector.AppendColumns(&cols)
				bundleCollector.AppendColumns(&cols)
				in.CacheWriter.Save(in.CacheDir, key, cols)
				continue
			}
		}
		cols, pdur, rdur := p.runManifestOne(in, path)
		parseDur += pdur
		ruleDur += rdur
		collector.AppendColumns(&cols)
		if bundleCollector != nil {
			bundleCollector.AppendColumns(&cols)
		}
	}
	if bundleKey != "" && bundleCollector != nil {
		in.CacheWriter.Save(in.CacheDir, bundleKey, *bundleCollector.Columns())
	}
	return parseDur, ruleDur, stats
}

// runManifestOne does the parse + rule dispatch for a single manifest path.
// Returns the empty FindingColumns when parsing fails.
func (AndroidPhase) runManifestOne(in AndroidInput, path string) (scanner.FindingColumns, time.Duration, time.Duration) {
	parseStart := time.Now()
	parsed, err := android.ParseManifest(path)
	parseDur := time.Since(parseStart)
	if err != nil {
		return scanner.FindingColumns{}, parseDur, 0
	}
	manifest := ConvertManifestForRules(android.ConvertManifest(parsed, path))
	if in.Dispatcher == nil {
		return scanner.FindingColumns{}, parseDur, 0
	}
	ruleStart := time.Now()
	file := &scanner.File{
		Path:     path,
		Language: scanner.LangXML,
		Metadata: manifest,
	}
	cols := in.Dispatcher.RunManifest(file, manifest)
	return cols, parseDur, time.Since(ruleStart)
}

// resourceSourceCacheStats counts cache outcomes during the resource-source
// loop, one entry per source file.
type resourceSourceCacheStats struct {
	hits    int
	misses  int
	skips   int
	bundles int
}

// runResourceSourceRules dispatches resource-backed source rules over all
// source files once the merged ResourceIndex is available. Each per-file
// result is cached by (sourceContentHash, mergedResourceIndexFP) so warm
// runs skip the dispatcher entirely for unchanged files.
func (p AndroidPhase) runResourceSourceRules(in AndroidInput, resourceSourceIndexes []*android.ResourceIndex, mergedFP string, collector *scanner.FindingCollector) (time.Duration, resourceSourceCacheStats) {
	var stats resourceSourceCacheStats
	if in.Dispatcher == nil || len(in.SourceFiles) == 0 || len(resourceSourceIndexes) == 0 {
		return 0, stats
	}
	start := time.Now()
	mergedSourceIdx := android.MergeResourceIndexes(resourceSourceIndexes...)
	cacheable := in.CacheDir != "" && in.CacheWriter != nil && in.RuleHash != "" && in.Project != nil

	if cacheable && mergedFP == "" {
		mergedFP = mergedResourceIndexFingerprint(in.Project.ResDirs)
	}
	if cacheable {
		resourceDeps := p.androidResourceDeps(in.ActiveRules)
		_ = saveMergedResourceIndexBundle(in.CacheDir, in.resourceSourceIndexBundleKey(mergedFP, resourceDeps, androidValuesScanKinds(resourceDeps)), mergedSourceIdx)
	}
	if cacheable {
		if handled, deltaStats := tryResourceSourceBundleDelta(in, mergedSourceIdx, mergedFP, collector); handled {
			return time.Since(start), deltaStats
		}
	}
	var bundleCollector *scanner.FindingCollector
	if cacheable {
		bundleCollector = scanner.NewFindingCollector(len(in.SourceFiles))
	}
	memo := hashutil.Default()
	shortHashEntries := make([]resourceSourceEntry, 0, len(in.SourceFiles))
	fullHashes := make(map[string]string, len(in.SourceFiles))
	for _, file := range in.SourceFiles {
		if cacheable && runCachedResourceSourceRule(in, file, mergedSourceIdx, mergedFP, memo, collector, bundleCollector, &shortHashEntries, fullHashes, &stats) {
			continue
		}
		cols := in.Dispatcher.RunResourceSource(file, mergedSourceIdx)
		collector.AppendColumns(&cols)
		if bundleCollector != nil {
			bundleCollector.AppendColumns(&cols)
		}
	}
	if cacheable {
		saveResourceSourceBundles(in, mergedFP, bundleCollector, shortHashEntries, fullHashes)
	}
	return time.Since(start), stats
}

func saveResourceSourceBundles(in AndroidInput, mergedFP string, bundleCollector *scanner.FindingCollector, shortHashEntries []resourceSourceEntry, fullHashes map[string]string) {
	if sourceSetFP, ok := resourceSourceSetFingerprint(in.SourceFiles); ok {
		bundleKey := in.resourceSourceBundleKey(sourceSetFP, mergedFP)
		in.CacheWriter.Save(in.CacheDir, bundleKey, *bundleCollector.Columns())
		manifestHashes := fullHashes
		if hashes, ok := resourceSourceHashes(in); ok {
			manifestHashes = hashes
		}
		if paths := sourcePathsForManifest(in); len(manifestHashes) == len(paths) {
			if manifestKey, ok := in.resourceSourceBundleManifestKey(paths, mergedFP); ok {
				_ = saveResourceSourceBundleManifest(in.CacheDir, manifestKey, bundleKey, manifestHashes)
			}
		}
	}
	if sourceSetFP, ok := resourceSourceEntriesFingerprint(shortHashEntries); ok && len(shortHashEntries) == len(in.SourceFiles) {
		in.CacheWriter.Save(in.CacheDir, in.resourceSourceBundleKey(sourceSetFP, mergedFP), *bundleCollector.Columns())
	}
}

func runCachedResourceSourceRule(in AndroidInput, file *scanner.File, mergedSourceIdx *android.ResourceIndex, mergedFP string, memo *hashutil.Memo, collector, bundleCollector *scanner.FindingCollector, shortHashEntries *[]resourceSourceEntry, fullHashes map[string]string, stats *resourceSourceCacheStats) bool {
	var provider func() ([]byte, error)
	if len(file.Content) > 0 {
		content := file.Content
		provider = func() ([]byte, error) { return content, nil }
	}
	srcHash, err := memo.HashFile(file.Path, provider)
	if err != nil {
		stats.skips++
		return false
	}
	if len(srcHash) >= 16 {
		*shortHashEntries = append(*shortHashEntries, resourceSourceEntry{path: file.Path, hash: srcHash[:16]})
	}
	fullHashes[file.Path] = srcHash
	key := in.resourceSourceKey(file.Path, srcHash, mergedFP)
	if cached, ok := scanner.LoadAndroidFindings(in.CacheDir, key); ok {
		collector.AppendColumns(&cached)
		bundleCollector.AppendColumns(&cached)
		stats.hits++
		return true
	}
	stats.misses++
	cols := in.Dispatcher.RunResourceSource(file, mergedSourceIdx)
	collector.AppendColumns(&cols)
	bundleCollector.AppendColumns(&cols)
	in.CacheWriter.Save(in.CacheDir, key, cols)
	return true
}

func tryResourceSourceBundleDelta(in AndroidInput, mergedSourceIdx *android.ResourceIndex, mergedFP string, collector *scanner.FindingCollector) (bool, resourceSourceCacheStats) {
	var stats resourceSourceCacheStats
	paths := sourcePathsForManifest(in)
	manifestKey, ok := in.resourceSourceBundleManifestKey(paths, mergedFP)
	if !ok {
		return false, stats
	}
	manifest, ok := loadResourceSourceBundleManifest(in.CacheDir, manifestKey)
	if !ok {
		return false, stats
	}
	previous, ok := scanner.LoadAndroidFindings(in.CacheDir, manifest.BundleKey)
	if !ok {
		return false, stats
	}
	currentHashes, ok := resourceSourceHashes(in)
	if !ok || len(currentHashes) != len(manifest.Hashes) {
		return false, stats
	}
	var changed []*scanner.File
	var changedPaths []string
	sourceByPath := sourceFileByPath(in.SourceFiles)
	for _, path := range paths {
		if path == "" {
			return false, stats
		}
		if manifest.Hashes[path] != currentHashes[path] {
			file := sourceByPath[path]
			if file == nil {
				return false, stats
			}
			changed = append(changed, file)
			changedPaths = append(changedPaths, path)
		}
	}
	if len(changed) == 0 || len(changed) > 16 {
		return false, stats
	}
	replacementCollector := scanner.NewFindingCollector(len(changed))
	for _, file := range changed {
		cols := in.Dispatcher.RunResourceSource(file, mergedSourceIdx)
		replacementCollector.AppendColumns(&cols)
		srcHash := currentHashes[file.Path]
		if srcHash != "" {
			in.CacheWriter.Save(in.CacheDir, in.resourceSourceKey(file.Path, srcHash, mergedFP), cols)
		}
	}
	replacement := *replacementCollector.Columns()
	merged := scanner.ApplyDelta(&previous, &replacement, changedPaths)
	collector.AppendColumns(&merged)
	if sourceSetFP, ok := resourceSourceEntriesFingerprint(resourceSourceEntriesFromHashes(currentHashes)); ok {
		bundleKey := in.resourceSourceBundleKey(sourceSetFP, mergedFP)
		in.CacheWriter.Save(in.CacheDir, bundleKey, merged)
		_ = saveResourceSourceBundleManifest(in.CacheDir, manifestKey, bundleKey, currentHashes)
	}
	stats.hits = len(paths) - len(changed)
	stats.misses = len(changed)
	stats.bundles = 1
	return true, stats
}

func sourcePathsForManifest(in AndroidInput) []string {
	// Build the set of paths we have hashable evidence for: a SourceFile or
	// a precomputed SourceHash. Anything else can't take part in the manifest
	// (its hash can't be recomputed at load time), so excluding it keeps the
	// save-side and load-side path lists in lockstep.
	have := make(map[string]struct{}, len(in.SourceFiles)+len(in.SourceHashes))
	for _, file := range in.SourceFiles {
		if file != nil && file.Path != "" {
			have[file.Path] = struct{}{}
		}
	}
	for path, hash := range in.SourceHashes {
		if path != "" && hash != "" {
			have[path] = struct{}{}
		}
	}
	src := in.SourcePaths
	if len(src) == 0 {
		src = make([]string, 0, len(have))
		for path := range have {
			src = append(src, path)
		}
	}
	seen := make(map[string]struct{}, len(src))
	out := make([]string, 0, len(src))
	for _, p := range src {
		if p == "" {
			continue
		}
		if _, ok := have[p]; !ok {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func resourceSourceHashes(in AndroidInput) (map[string]string, bool) {
	memo := hashutil.Default()
	paths := sourcePathsForManifest(in)
	hashes := make(map[string]string, len(paths))
	sourceByPath := sourceFileByPath(in.SourceFiles)
	for _, path := range paths {
		if path == "" {
			return nil, false
		}
		if hash := in.SourceHashes[path]; hash != "" {
			hashes[path] = hash
			continue
		}
		file := sourceByPath[path]
		if file == nil {
			return nil, false
		}
		var provider func() ([]byte, error)
		if len(file.Content) > 0 {
			content := file.Content
			provider = func() ([]byte, error) { return content, nil }
		}
		hash, err := memo.HashFile(path, provider)
		if err != nil {
			return nil, false
		}
		hashes[path] = hash
	}
	return hashes, true
}

func sourceFileByPath(files []*scanner.File) map[string]*scanner.File {
	out := make(map[string]*scanner.File, len(files))
	for _, file := range files {
		if file == nil || file.Path == "" {
			continue
		}
		out[file.Path] = file
	}
	return out
}

func resourceSourceEntriesFromHashes(hashes map[string]string) []resourceSourceEntry {
	entries := make([]resourceSourceEntry, 0, len(hashes))
	for path, hash := range hashes {
		entries = append(entries, resourceSourceEntry{path: path, hash: hash})
	}
	return entries
}

// gradleCacheStats counts cache outcomes during the gradle loop.
type gradleCacheStats struct {
	hits   int
	misses int
	skips  int
}

func (AndroidPhase) gradleBundleFingerprint(paths []string) (string, bool) {
	h := hashutil.Hasher().New()
	if !writePathHashes(h, "gradle", paths) {
		return "", false
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

// runGradlePhase iterates over all Gradle paths, parses each (or loads
// cached findings), dispatches Gradle rules on miss, and queues fresh
// entries on the writer. Cross-file context — version catalogs and
// dependency closures — is captured in libraryFactsFP, which is part of
// every cache key. So a `libs.versions.toml` edit invalidates every
// Gradle entry; a single build script edit only invalidates that one.
func (p AndroidPhase) runGradlePhase(in AndroidInput, collector *scanner.FindingCollector) (readParseDur, rulesDur time.Duration, stats gradleCacheStats) {
	cacheable := in.CacheDir != "" && in.CacheWriter != nil && in.RuleHash != ""
	var bundleKey string
	var bundleCollector *scanner.FindingCollector
	if cacheable {
		if bundleFP, ok := p.gradleBundleFingerprint(in.Project.GradlePaths); ok {
			bundleKey = in.gradleBundleKey(bundleFP)
			if cached, hit := scanner.LoadAndroidFindings(in.CacheDir, bundleKey); hit {
				collector.AppendColumns(&cached)
				stats.hits = len(in.Project.GradlePaths)
				return 0, 0, stats
			}
			bundleCollector = scanner.NewFindingCollector(len(in.Project.GradlePaths) * 4)
		}
	}
	memo := hashutil.Default()
	for _, path := range in.Project.GradlePaths {
		if cacheable {
			contentHash, err := memo.HashFile(path, nil)
			if err != nil {
				stats.skips++
			} else {
				key := in.gradleKey(path, contentHash)
				if cached, ok := scanner.LoadAndroidFindings(in.CacheDir, key); ok {
					collector.AppendColumns(&cached)
					if bundleCollector != nil {
						bundleCollector.AppendColumns(&cached)
					}
					stats.hits++
					continue
				}
				stats.misses++
				cols, rpdur, rdur := p.runGradleOne(in, path)
				readParseDur += rpdur
				rulesDur += rdur
				collector.AppendColumns(&cols)
				if bundleCollector != nil {
					bundleCollector.AppendColumns(&cols)
				}
				in.CacheWriter.Save(in.CacheDir, key, cols)
				continue
			}
		}
		cols, rpdur, rdur := p.runGradleOne(in, path)
		readParseDur += rpdur
		rulesDur += rdur
		collector.AppendColumns(&cols)
		if bundleCollector != nil {
			bundleCollector.AppendColumns(&cols)
		}
	}
	if bundleKey != "" && bundleCollector != nil {
		in.CacheWriter.Save(in.CacheDir, bundleKey, *bundleCollector.Columns())
	}
	return readParseDur, rulesDur, stats
}

// runGradleOne reads, parses, and dispatches Gradle rules for a single
// build script. Returns empty columns when the file can't be read or
// parsed.
func (AndroidPhase) runGradleOne(in AndroidInput, path string) (scanner.FindingColumns, time.Duration, time.Duration) {
	readParseStart := time.Now()
	content, err := os.ReadFile(path)
	if err != nil {
		return scanner.FindingColumns{}, time.Since(readParseStart), 0
	}
	cfg, err := android.ParseBuildGradleContent(string(content))
	readParseDur := time.Since(readParseStart)
	if err != nil {
		return scanner.FindingColumns{}, readParseDur, 0
	}
	if in.Dispatcher == nil {
		return scanner.FindingColumns{}, readParseDur, 0
	}
	ruleStart := time.Now()
	file := &scanner.File{
		Path:     path,
		Language: scanner.LangGradle,
		Content:  content,
		Metadata: cfg,
	}
	cols := in.Dispatcher.RunGradle(file, cfg)
	return cols, readParseDur, time.Since(ruleStart)
}

func (p AndroidPhase) runManifestSubphase(in AndroidInput, collector *scanner.FindingCollector, tracker perf.Tracker) {
	manifestTracker := tracker.Serial("manifestAnalysis")
	manifestParseDur, manifestRuleDur, manifestStats := p.runManifestPhase(in, collector)
	perf.AddEntry(manifestTracker, "manifestParse", manifestParseDur)
	perf.AddEntry(manifestTracker, "manifestRuleChecks", manifestRuleDur)
	if manifestStats.hits+manifestStats.misses+manifestStats.skips > 0 {
		perf.AddEntryDetails(manifestTracker, "manifestCache", 0, map[string]int64{
			"hits":   int64(manifestStats.hits),
			"misses": int64(manifestStats.misses),
			"skips":  int64(manifestStats.skips),
		}, nil)
	}
	manifestTracker.End()
}

func (p AndroidPhase) runResourceSubphase(in AndroidInput, resourceDeps rules.AndroidDataDependency, valueKinds android.ValuesScanKind, needResources bool, needIcons bool, providers *AndroidProjectProviders, collector *scanner.FindingCollector, tracker perf.Tracker) {
	resourceTracker := tracker.Serial("resourceAnalysis")
	var t androidResourceTimings
	var resourceStats resourceCacheStats
	var resourceSourceIndexes []*android.ResourceIndex
	var resourceBundleCollector *scanner.FindingCollector
	resDirFPs := newResDirFingerprints(in.CacheDir, len(in.Project.ResDirs))
	androidFindingsCacheable := in.CacheDir != "" && in.CacheWriter != nil && in.RuleHash != ""
	needResourceSource := hasResourceSourceRules(in.ActiveRules) && (len(in.SourceFiles) > 0 || len(in.SourcePaths) > 0)
	if p.tryWarmResourceSubphase(in, resourceDeps, valueKinds, needResources, needIcons, needResourceSource, androidFindingsCacheable, &resDirFPs, collector, resourceTracker) {
		return
	}
	warmResourceIndexHit := false
	if idx, stats, ok := p.tryWarmResourceIndexBundle(in, resourceDeps, valueKinds, needResources, needIcons, needResourceSource, androidFindingsCacheable, &resDirFPs, collector); ok {
		resourceSourceIndexes = append(resourceSourceIndexes, idx)
		resourceStats = stats
		warmResourceIndexHit = true
	}
	if !warmResourceIndexHit {
		if needResources && androidFindingsCacheable {
			resourceBundleCollector = scanner.NewFindingCollector(len(in.Project.ResDirs) * 8)
		}
		for _, resDir := range in.Project.ResDirs {
			var resDirFP string
			if needResources && androidFindingsCacheable {
				resDirFP = resDirFPs.fingerprint(resDir)
			}
			mergedIdx := p.runResourceDir(in, resDir, resDirFP, resourceDeps, valueKinds, needIcons, needResourceSource, providers, collector, resourceBundleCollector, &t, &resourceStats)
			if mergedIdx != nil {
				resourceSourceIndexes = append(resourceSourceIndexes, mergedIdx)
			}
		}
		if resourceBundleCollector != nil {
			mergedResourceFP := mergedResourceIndexFingerprintWith(in.Project.ResDirs, &resDirFPs)
			in.CacheWriter.Save(in.CacheDir, in.resourceBundleKey(mergedResourceFP, resourceDeps, valueKinds), *resourceBundleCollector.Columns())
		}
	}
	perf.AddEntry(resourceTracker, "resourceDirScan", t.waitDur)
	perf.AddEntry(resourceTracker, "resourceDirScanCPU", t.scanDur)
	perf.AddEntry(resourceTracker, "layoutDirScan", t.layoutScanDur)
	perf.AddEntry(resourceTracker, "valuesDirScanCPU", t.valuesScanDur)
	perf.AddEntry(resourceTracker, "valuesFileRead", t.valuesReadDur)
	perf.AddEntry(resourceTracker, "valuesXMLParseCPU", t.valuesParseDur)
	perf.AddEntry(resourceTracker, "valuesIndexBuild", t.valuesIndexDur)
	perf.AddEntry(resourceTracker, "resourceMerge", t.mergeDur)
	perf.AddEntry(resourceTracker, "maxLayoutDirScan", t.maxLayoutDur)
	perf.AddEntry(resourceTracker, "maxValuesDirScan", t.maxValuesDur)
	perf.AddEntry(resourceTracker, "maxDrawableDirScan", t.maxDrawableDur)
	perf.AddEntry(resourceTracker, "drawableDirScan", t.drawableScanDur)
	perf.AddEntry(resourceTracker, "resourceRuleChecks", t.rulesDur)
	if resourceStats.hits+resourceStats.misses+resourceStats.skips > 0 {
		perf.AddEntryDetails(resourceTracker, "resourceCache", 0, map[string]int64{
			"hits":   int64(resourceStats.hits),
			"misses": int64(resourceStats.misses),
			"skips":  int64(resourceStats.skips),
		}, nil)
	}
	if needResourceSource {
		p.runResourceSourceSubphase(in, resourceSourceIndexes, resDirFPs, androidFindingsCacheable, collector, resourceTracker, &t)
	}
	perf.AddEntry(resourceTracker, "iconScan", t.iconWaitDur)
	perf.AddEntry(resourceTracker, "iconScanCPU", t.iconScanDur)
	perf.AddEntry(resourceTracker, "iconRuleChecks", t.iconRulesDur)
	resourceTracker.End()
}

func (p AndroidPhase) tryWarmResourceSubphase(in AndroidInput, resourceDeps rules.AndroidDataDependency, valueKinds android.ValuesScanKind, needResources, needIcons, needResourceSource, cacheable bool, resDirFPs *resDirFingerprints, collector *scanner.FindingCollector, resourceTracker perf.Tracker) bool {
	if !cacheable || !needResourceSource || needIcons || in.Project == nil || in.Dispatcher == nil {
		return false
	}
	var sourceStats resourceSourceCacheStats
	warmCollector := scanner.NewFindingCollector(len(in.Project.ResDirs) * 8)
	resourceStats, ok := in.loadWarmResourceFindings(resourceDeps, valueKinds, needResources, resDirFPs, warmCollector)
	if !ok {
		return false
	}
	mergedResourceFP := mergedResourceIndexFingerprintWith(in.Project.ResDirs, resDirFPs)
	sourceSetFP, sourceCount, ok := in.warmResourceSourceBundleFingerprint()
	if !ok {
		return false
	}
	cached, ok := scanner.LoadAndroidFindings(in.CacheDir, in.resourceSourceBundleKey(sourceSetFP, mergedResourceFP))
	if !ok {
		return false
	}
	warmCollector.AppendColumns(&cached)
	collector.AppendColumns(warmCollector.Columns())
	sourceStats.hits = sourceCount
	sourceStats.bundles = 1
	perf.AddEntry(resourceTracker, "resourceDirScan", 0)
	perf.AddEntry(resourceTracker, "resourceDirScanCPU", 0)
	perf.AddEntry(resourceTracker, "layoutDirScan", 0)
	perf.AddEntry(resourceTracker, "valuesDirScanCPU", 0)
	perf.AddEntry(resourceTracker, "valuesFileRead", 0)
	perf.AddEntry(resourceTracker, "valuesXMLParseCPU", 0)
	perf.AddEntry(resourceTracker, "valuesIndexBuild", 0)
	perf.AddEntry(resourceTracker, "resourceMerge", 0)
	perf.AddEntry(resourceTracker, "maxLayoutDirScan", 0)
	perf.AddEntry(resourceTracker, "maxValuesDirScan", 0)
	perf.AddEntry(resourceTracker, "maxDrawableDirScan", 0)
	perf.AddEntry(resourceTracker, "drawableDirScan", 0)
	perf.AddEntry(resourceTracker, "resourceRuleChecks", 0)
	if resourceStats.hits+resourceStats.misses+resourceStats.skips > 0 {
		perf.AddEntryDetails(resourceTracker, "resourceCache", 0, map[string]int64{
			"hits":   int64(resourceStats.hits),
			"misses": int64(resourceStats.misses),
			"skips":  int64(resourceStats.skips),
		}, nil)
	}
	perf.AddEntry(resourceTracker, "resourceSourceRuleChecks", 0)
	perf.AddEntryDetails(resourceTracker, "resourceSourceCache", 0, map[string]int64{
		"hits":    int64(sourceStats.hits),
		"misses":  int64(sourceStats.misses),
		"skips":   int64(sourceStats.skips),
		"bundles": int64(sourceStats.bundles),
	}, nil)
	perf.AddEntry(resourceTracker, "iconScan", 0)
	perf.AddEntry(resourceTracker, "iconScanCPU", 0)
	perf.AddEntry(resourceTracker, "iconRuleChecks", 0)
	resourceTracker.End()
	return true
}

func (in AndroidInput) loadWarmResourceFindings(resourceDeps rules.AndroidDataDependency, valueKinds android.ValuesScanKind, needResources bool, resDirFPs *resDirFingerprints, warmCollector *scanner.FindingCollector) (resourceCacheStats, bool) {
	var resourceStats resourceCacheStats
	if !needResources || resourceDeps&(rules.AndroidDepLayout|rules.AndroidDepValues) == 0 {
		return resourceStats, true
	}
	for _, resDir := range in.Project.ResDirs {
		resDirFP := resDirFPs.fingerprint(resDir)
		resourceKey := in.resourceKey(resDir, resDirFP, resourceDeps, valueKinds)
		cached, ok := scanner.LoadAndroidFindings(in.CacheDir, resourceKey)
		if !ok {
			return resourceStats, false
		}
		warmCollector.AppendColumns(&cached)
		resourceStats.hits++
	}
	return resourceStats, true
}

func (in AndroidInput) warmResourceSourceBundleFingerprint() (string, int, bool) {
	if len(in.SourceFiles) > 0 {
		if sourceSetFP, ok := resourceSourceSetFingerprint(in.SourceFiles); ok {
			return sourceSetFP, len(in.SourceFiles), true
		}
	}
	if len(in.SourcePaths) > 0 && len(in.SourceHashes) > 0 {
		sourceSetFP, ok := resourceSourceEntriesFingerprintFromHashes(in.SourcePaths, in.SourceHashes)
		if ok {
			return sourceSetFP, len(in.SourcePaths), true
		}
	}
	if len(in.SourcePaths) > 0 {
		sourceSetFP, ok := resourceSourceSetFingerprintFromPaths(in.SourcePaths)
		return sourceSetFP, len(in.SourcePaths), ok
	}
	return "", len(in.SourceFiles), false
}

func (p AndroidPhase) tryWarmResourceIndexBundle(in AndroidInput, resourceDeps rules.AndroidDataDependency, valueKinds android.ValuesScanKind, needResources, needIcons, needResourceSource, cacheable bool, resDirFPs *resDirFingerprints, collector *scanner.FindingCollector) (*android.ResourceIndex, resourceCacheStats, bool) {
	var stats resourceCacheStats
	if !cacheable || !needResourceSource || needIcons || in.Project == nil || in.Dispatcher == nil || len(in.SourceFiles) == 0 {
		return nil, stats, false
	}
	warmCollector := scanner.NewFindingCollector(len(in.Project.ResDirs) * 8)
	mergedResourceFP := mergedResourceIndexFingerprintWith(in.Project.ResDirs, resDirFPs)
	if needResources && resourceDeps&(rules.AndroidDepLayout|rules.AndroidDepValues) != 0 {
		if cached, ok := scanner.LoadAndroidFindings(in.CacheDir, in.resourceBundleKey(mergedResourceFP, resourceDeps, valueKinds)); ok {
			warmCollector.AppendColumns(&cached)
			stats.hits = len(in.Project.ResDirs)
		} else {
			for _, resDir := range in.Project.ResDirs {
				resDirFP := resDirFPs.fingerprint(resDir)
				resourceKey := in.resourceKey(resDir, resDirFP, resourceDeps, valueKinds)
				cached, ok := scanner.LoadAndroidFindings(in.CacheDir, resourceKey)
				if !ok {
					return nil, stats, false
				}
				warmCollector.AppendColumns(&cached)
				stats.hits++
			}
		}
	}
	idx, ok := loadMergedResourceIndexBundle(in.CacheDir, in.resourceSourceIndexBundleKey(mergedResourceFP, resourceDeps, valueKinds))
	if !ok {
		return nil, stats, false
	}
	collector.AppendColumns(warmCollector.Columns())
	return idx, stats, true
}

func (p AndroidPhase) runResourceSourceSubphase(in AndroidInput, resourceSourceIndexes []*android.ResourceIndex, resDirFPs resDirFingerprints, cacheable bool, collector *scanner.FindingCollector, resourceTracker perf.Tracker, t *androidResourceTimings) {
	var sourceStats resourceSourceCacheStats
	var mergedResourceFP string
	if cacheable && len(in.SourceFiles) > 0 && len(resourceSourceIndexes) > 0 {
		mergedResourceFP = mergedResourceIndexFingerprintWith(in.Project.ResDirs, &resDirFPs)
	}
	t.sourceRulesDur, sourceStats = p.runResourceSourceRules(in, resourceSourceIndexes, mergedResourceFP, collector)
	perf.AddEntry(resourceTracker, "resourceSourceRuleChecks", t.sourceRulesDur)
	if sourceStats.hits+sourceStats.misses+sourceStats.skips > 0 {
		perf.AddEntryDetails(resourceTracker, "resourceSourceCache", 0, map[string]int64{
			"hits":    int64(sourceStats.hits),
			"misses":  int64(sourceStats.misses),
			"skips":   int64(sourceStats.skips),
			"bundles": int64(sourceStats.bundles),
		}, nil)
	}
}

func (p AndroidPhase) runGradleSubphase(in AndroidInput, collector *scanner.FindingCollector, tracker perf.Tracker) {
	gradleTracker := tracker.Serial("gradleAnalysis")
	gradleReadParseDur, gradleRulesDur, gradleStats := p.runGradlePhase(in, collector)
	perf.AddEntry(gradleTracker, "gradleReadParse", gradleReadParseDur)
	perf.AddEntry(gradleTracker, "gradleRuleChecks", gradleRulesDur)
	if gradleStats.hits+gradleStats.misses+gradleStats.skips > 0 {
		perf.AddEntryDetails(gradleTracker, "gradleCache", 0, map[string]int64{
			"hits":   int64(gradleStats.hits),
			"misses": int64(gradleStats.misses),
			"skips":  int64(gradleStats.skips),
		}, nil)
	}
	gradleTracker.End()
}

// Run implements Phase.
func (p AndroidPhase) Run(ctx context.Context, in AndroidInput) (AndroidResult, error) {
	if err := ctx.Err(); err != nil {
		return AndroidResult{}, err
	}
	if in.Project == nil || in.Project.IsEmpty() {
		return AndroidResult{}, nil
	}

	needs := classifyAndroidPhaseNeeds(in.ActiveRules)
	if !needs.any() {
		return AndroidResult{}, nil
	}

	collector := scanner.NewFindingCollector(len(in.Project.ManifestPaths)*4 + len(in.Project.ResDirs)*8 + len(in.Project.GradlePaths)*4)

	resourceDeps := p.androidResourceDeps(in.ActiveRules)
	valueKinds := androidValuesScanKinds(resourceDeps)
	providers := in.Providers
	if providers == nil {
		providers = NewAndroidProjectProviders(in.Project, CollectAndroidDependenciesV2(in.ActiveRules), runtime.NumCPU())
	}

	tracker := in.Tracker
	if tracker == nil {
		tracker = perf.New(false)
	}

	projectBundleKey := ""
	if in.CacheDir != "" && in.RuleHash != "" && len(in.SourceFiles) == 0 {
		resDirFPs := newResDirFingerprints(in.CacheDir, len(in.Project.ResDirs))
		if key, ok := in.androidProjectBundleKey(needs, resourceDeps, valueKinds, &resDirFPs); ok {
			if cached, hit := scanner.LoadAndroidFindings(in.CacheDir, key); hit {
				perf.AddEntryDetails(tracker, "androidProjectCache", 0, map[string]int64{
					"hits": 1,
				}, nil)
				return AndroidResult{Findings: cached}, nil
			}
			projectBundleKey = key
			perf.AddEntryDetails(tracker, "androidProjectCache", 0, map[string]int64{
				"misses": 1,
			}, nil)
		}
	}

	if needs.manifest {
		p.runManifestSubphase(in, collector, tracker)
	}

	if needs.resources || needs.icons {
		p.runResourceSubphase(in, resourceDeps, valueKinds, needs.resources, needs.icons, providers, collector, tracker)
	}

	if needs.gradle {
		p.runGradleSubphase(in, collector, tracker)
	}

	cols := *collector.Columns()
	if projectBundleKey != "" && in.CacheWriter != nil {
		in.CacheWriter.Save(in.CacheDir, projectBundleKey, cols)
	}
	return AndroidResult{Findings: cols}, nil
}

// AndroidProjectProviders bundles the async scan futures for manifests,
// resources, and icons so IndexPhase can start them in parallel with
// Kotlin parsing and AndroidPhase can await them once the per-file pass
// is done.
type AndroidProjectProviders struct {
	project *android.Project
	deps    rules.AndroidDataDependency

	valuesFutures map[string]*android.ResourceScanFuture
	layoutFutures map[string]*android.ResourceScanFuture
	iconFutures   map[string]*android.IconScanFuture
}

// NewAndroidProjectProviders constructs a provider bundle for the given
// Android project and rule dependency mask. The bundle's futures are
// created (but not started) and sized via maxWorkers — call Start() to
// kick them off.
func NewAndroidProjectProviders(project *android.Project, deps rules.AndroidDataDependency, maxWorkers int) *AndroidProjectProviders {
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

// CollectAndroidDependenciesV2 reads AndroidDeps directly from
// api.Rule.AndroidDeps.
func CollectAndroidDependenciesV2(activeRules []*api.Rule) rules.AndroidDataDependency {
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

func androidIconProviderStartConcurrency(_ int) int {
	return 1
}

func androidValuesScanKinds(deps rules.AndroidDataDependency) android.ValuesScanKind {
	var kinds android.ValuesScanKind
	if deps&rules.AndroidDepValuesStrings != 0 {
		kinds |= android.ValuesScanStrings
	}
	if deps&rules.AndroidDepValuesDimensions != 0 {
		kinds |= android.ValuesScanDimensions
	}
	if deps&rules.AndroidDepValuesPlurals != 0 {
		kinds |= android.ValuesScanPlurals
	}
	if deps&rules.AndroidDepValuesArrays != 0 {
		kinds |= android.ValuesScanArrays
	}
	if deps&rules.AndroidDepValuesExtraText != 0 {
		kinds |= android.ValuesScanExtraText
	}
	return kinds
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
