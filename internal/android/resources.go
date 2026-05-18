package android

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kaeawc/krit/internal/scanner"
)

func clampWorkers(maxWorkers, workItems int) int {
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	if workItems < 1 {
		return 1
	}
	if maxWorkers > workItems {
		return workItems
	}
	return maxWorkers
}

// ExtraTextEntry records stray text found between elements in a values XML file.
type ExtraTextEntry struct {
	FilePath string
	Line     int
	Text     string
}

// ResourceIndex holds all parsed Android resource data from a res/ directory.
type ResourceIndex struct {
	Layouts             map[string]*Layout            // layout name -> parsed layout (last wins for compat)
	LayoutConfigs       map[string]map[string]*Layout // layout name -> config qualifier -> layout
	Strings             map[string]string             // string name -> value
	StringsNonTranslate map[string]bool               // strings marked translatable="false"
	// StringsNonFormatted marks strings with formatted="false". These are
	// NOT format strings — their `%{var}` / `%foo` sequences are literal,
	// typically for gettext/phrase-style translation interpolation, and
	// format-specifier-validation rules should skip them.
	StringsNonFormatted map[string]bool
	// StringsTrailingWS marks strings whose raw XML text content ended with
	// whitespace before TrimSpace. Significant in some locales and in
	// concatenated strings.
	StringsTrailingWS  map[string]bool
	StringsLocation    map[string]StringLocation    // string name -> file path and line
	Colors             map[string]string            // color name -> value
	Dimensions         map[string]string            // dimen name -> value
	DimensionsLocation map[string]StringLocation    // dimen name -> file path and line
	Styles             map[string]*Style            // style name -> style definition
	Drawables          []string                     // drawable resource names
	DrawableSelectors  map[string][]SelectorItem    // selector drawable name -> child items
	StringArrays       map[string][]string          // string-array name -> items
	Plurals            map[string]map[string]string // plural name -> quantity -> value
	Integers           map[string]string            // integer name -> value
	Booleans           map[string]string            // bool name -> value
	IDs                map[string]bool              // declared @+id names
	ExtraTexts         []ExtraTextEntry             // stray text nodes in values files
}

// SelectorItem records a child <item> in a drawable selector XML file.
type SelectorItem struct {
	FilePath   string
	Line       int
	StateAttrs map[string]string
}

// StringLocation records where a <string> element was defined.
type StringLocation struct {
	FilePath string
	Line     int
}

// Layout represents a parsed Android layout XML file.
type Layout struct {
	Name     string
	FilePath string // absolute path to the layout XML file
	RootView *View
}

// View represents a single view element in a layout hierarchy.
type View struct {
	Type               string            // e.g., "TextView", "LinearLayout"
	ID                 string            // android:id value (e.g., "@+id/title")
	ContentDescription string            // android:contentDescription
	Text               string            // android:text
	LayoutWidth        string            // android:layout_width
	LayoutHeight       string            // android:layout_height
	Background         string            // android:background
	Attributes         map[string]string // all attributes
	Children           []*View
	Line               int // 1-based line number in the XML file
}

// Style represents an Android style definition.
type Style struct {
	Name      string
	Parent    string
	FilePath  string            // absolute path to the values XML file containing the style
	Line      int               // 1-based line of the <style> element
	Items     map[string]string // item name -> value
	ItemLines map[string]int    // item name -> 1-based line of the <item> element
}

// ResourceScanStats captures where time is spent while indexing a res/ directory.
type ResourceScanStats struct {
	LayoutScanMs      int64
	ValuesScanMs      int64
	DrawableScanMs    int64
	ValuesReadMs      int64
	ValuesParseMs     int64
	ValuesIndexMs     int64
	MergeMs           int64
	MaxLayoutScanMs   int64
	MaxValuesScanMs   int64
	MaxDrawableScanMs int64
	LayoutDirCount    int64
	ValuesDirCount    int64
	DrawableDirCount  int64
}

type ValuesScanKind uint32

const (
	ValuesScanNone    ValuesScanKind = 0
	ValuesScanStrings ValuesScanKind = 1 << iota
	ValuesScanDimensions
	ValuesScanPlurals
	ValuesScanArrays
	ValuesScanExtraText
)

const ValuesScanAll = ValuesScanStrings | ValuesScanDimensions | ValuesScanPlurals | ValuesScanArrays | ValuesScanExtraText

// lineAtOffset returns the 1-based line number for the given byte offset in
// the file's content. A nil file is treated as the first line.
func lineAtOffset(file *scanner.File, offset int64) int {
	if file == nil {
		return 1
	}
	return file.RowForByte(int(offset)) + 1
}

type resourceDirResult struct {
	order int
	index *ResourceIndex
	stats ResourceScanStats
	err   error
}

// ResourceValueProvider materializes values XML resources into a ResourceIndex.
// Providers may load eagerly or lazily, but must preserve the current merged
// output semantics when applied.
type ResourceValueProvider interface {
	LoadInto(idx *ResourceIndex) (ResourceScanStats, error)
}

// LazyValuesScan starts values parsing on demand and caches the merged result
// so callers can overlap work and apply the parsed values later.
type LazyValuesScan struct {
	dir        string
	maxWorkers int
	kinds      ValuesScanKind

	once  sync.Once
	done  chan struct{}
	index *ResourceIndex
	stats ResourceScanStats
	err   error
}

// NewLazyValuesScan returns a reusable provider for a single values/ directory.
func NewLazyValuesScan(dir string, maxWorkers int, kinds ...ValuesScanKind) *LazyValuesScan {
	kindMask := ValuesScanAll
	if len(kinds) > 0 && kinds[0] != ValuesScanNone {
		kindMask = kinds[0]
	}
	return &LazyValuesScan{
		dir:        dir,
		maxWorkers: maxWorkers,
		kinds:      kindMask,
		done:       make(chan struct{}),
	}
}

// Start begins the background values scan. Calling Start multiple times is safe.
func (p *LazyValuesScan) Start() {
	if p == nil {
		return
	}
	p.once.Do(func() {
		go func() {
			p.index, p.stats, p.err = scanValuesDirIndexKinds(p.dir, p.maxWorkers, p.kinds)
			close(p.done)
		}()
	})
}

// LoadInto waits for the values scan if needed and merges the results into idx.
func (p *LazyValuesScan) LoadInto(idx *ResourceIndex) (ResourceScanStats, error) {
	if p == nil {
		return ResourceScanStats{}, nil
	}
	p.Start()
	<-p.done
	if p.err != nil {
		return p.stats, p.err
	}
	idx.mergeFrom(p.index)
	return p.stats, nil
}

func newResourceIndex() *ResourceIndex {
	return &ResourceIndex{
		Layouts:             make(map[string]*Layout),
		LayoutConfigs:       make(map[string]map[string]*Layout),
		Strings:             make(map[string]string),
		StringsNonTranslate: make(map[string]bool),
		StringsNonFormatted: make(map[string]bool),
		StringsTrailingWS:   make(map[string]bool),
		StringsLocation:     make(map[string]StringLocation),
		Colors:              make(map[string]string),
		Dimensions:          make(map[string]string),
		DimensionsLocation:  make(map[string]StringLocation),
		Styles:              make(map[string]*Style),
		DrawableSelectors:   make(map[string][]SelectorItem),
		StringArrays:        make(map[string][]string),
		Plurals:             make(map[string]map[string]string),
		Integers:            make(map[string]string),
		Booleans:            make(map[string]string),
		IDs:                 make(map[string]bool),
	}
}

// MergeResourceIndexes combines multiple partial resource indexes into one.
func MergeResourceIndexes(indexes ...*ResourceIndex) *ResourceIndex {
	merged := newResourceIndex()
	for _, idx := range indexes {
		merged.mergeFrom(idx)
	}
	return merged
}

func (idx *ResourceIndex) mergeFrom(other *ResourceIndex) {
	if idx == nil || other == nil {
		return
	}
	idx.ensureMaps()
	idx.mergeLayoutEntries(other)
	idx.mergeStringEntries(other)
	idx.mergeColorDimenEntries(other)
	idx.mergeDrawableEntries(other)
	idx.mergeCollectionEntries(other)
	idx.ExtraTexts = append(idx.ExtraTexts, other.ExtraTexts...)
}

func (idx *ResourceIndex) ensureMaps() {
	if idx.Layouts == nil {
		idx.Layouts = make(map[string]*Layout)
	}
	if idx.LayoutConfigs == nil {
		idx.LayoutConfigs = make(map[string]map[string]*Layout)
	}
	if idx.Strings == nil {
		idx.Strings = make(map[string]string)
	}
	if idx.StringsNonTranslate == nil {
		idx.StringsNonTranslate = make(map[string]bool)
	}
	if idx.StringsNonFormatted == nil {
		idx.StringsNonFormatted = make(map[string]bool)
	}
	if idx.StringsTrailingWS == nil {
		idx.StringsTrailingWS = make(map[string]bool)
	}
	if idx.StringsLocation == nil {
		idx.StringsLocation = make(map[string]StringLocation)
	}
	if idx.Colors == nil {
		idx.Colors = make(map[string]string)
	}
	if idx.Dimensions == nil {
		idx.Dimensions = make(map[string]string)
	}
	if idx.DimensionsLocation == nil {
		idx.DimensionsLocation = make(map[string]StringLocation)
	}
	if idx.Styles == nil {
		idx.Styles = make(map[string]*Style)
	}
	if idx.StringArrays == nil {
		idx.StringArrays = make(map[string][]string)
	}
	if idx.Plurals == nil {
		idx.Plurals = make(map[string]map[string]string)
	}
	if idx.Integers == nil {
		idx.Integers = make(map[string]string)
	}
	if idx.Booleans == nil {
		idx.Booleans = make(map[string]string)
	}
	if idx.IDs == nil {
		idx.IDs = make(map[string]bool)
	}
}

func (idx *ResourceIndex) mergeLayoutEntries(other *ResourceIndex) {
	for name, layout := range other.Layouts {
		idx.Layouts[name] = layout
	}
	for name, configs := range other.LayoutConfigs {
		dst := idx.LayoutConfigs[name]
		if dst == nil {
			dst = make(map[string]*Layout, len(configs))
			idx.LayoutConfigs[name] = dst
		}
		for qualifier, layout := range configs {
			dst[qualifier] = layout
		}
	}
}

func (idx *ResourceIndex) mergeStringEntries(other *ResourceIndex) {
	for name, value := range other.Strings {
		idx.Strings[name] = value
	}
	for name := range other.StringsNonTranslate {
		idx.StringsNonTranslate[name] = true
	}
	for name := range other.StringsNonFormatted {
		if idx.StringsNonFormatted == nil {
			idx.StringsNonFormatted = make(map[string]bool)
		}
		idx.StringsNonFormatted[name] = true
	}
	for name := range other.StringsTrailingWS {
		if idx.StringsTrailingWS == nil {
			idx.StringsTrailingWS = make(map[string]bool)
		}
		idx.StringsTrailingWS[name] = true
	}
	for name, loc := range other.StringsLocation {
		idx.StringsLocation[name] = loc
	}
}

func (idx *ResourceIndex) mergeColorDimenEntries(other *ResourceIndex) {
	for name, value := range other.Colors {
		idx.Colors[name] = value
	}
	for name, value := range other.Dimensions {
		idx.Dimensions[name] = value
	}
	for name, loc := range other.DimensionsLocation {
		idx.DimensionsLocation[name] = loc
	}
	for name, style := range other.Styles {
		idx.Styles[name] = style
	}
	for name, value := range other.Integers {
		idx.Integers[name] = value
	}
	for name, value := range other.Booleans {
		idx.Booleans[name] = value
	}
	for name := range other.IDs {
		idx.IDs[name] = true
	}
}

func (idx *ResourceIndex) mergeDrawableEntries(other *ResourceIndex) {
	idx.Drawables = append(idx.Drawables, other.Drawables...)
	for name, items := range other.DrawableSelectors {
		if idx.DrawableSelectors == nil {
			idx.DrawableSelectors = make(map[string][]SelectorItem)
		}
		idx.DrawableSelectors[name] = append(idx.DrawableSelectors[name], items...)
	}
}

func (idx *ResourceIndex) mergeCollectionEntries(other *ResourceIndex) {
	// A locale overlay (e.g. values-fr/arrays.xml) may declare a collection's
	// shape without items (`<string-array name="colors"/>` or
	// `<plurals name="songs"/>`) when the translation is incomplete. Allowing
	// that empty overlay to overwrite the populated default would make rules
	// such as InconsistentArraysResource and MissingQuantityResource fire on
	// resources that ARE defined in the default configuration. Preserve the
	// populated entry instead.
	for name, items := range other.StringArrays {
		if len(items) == 0 {
			if existing, ok := idx.StringArrays[name]; ok && len(existing) > 0 {
				continue
			}
		}
		idx.StringArrays[name] = items
	}
	for name, quantities := range other.Plurals {
		if len(quantities) == 0 {
			if existing, ok := idx.Plurals[name]; ok && len(existing) > 0 {
				continue
			}
		}
		idx.Plurals[name] = quantities
	}
}

func (stats *ResourceScanStats) add(other ResourceScanStats) {
	stats.LayoutScanMs += other.LayoutScanMs
	stats.ValuesScanMs += other.ValuesScanMs
	stats.DrawableScanMs += other.DrawableScanMs
	stats.ValuesReadMs += other.ValuesReadMs
	stats.ValuesParseMs += other.ValuesParseMs
	stats.ValuesIndexMs += other.ValuesIndexMs
	stats.MergeMs += other.MergeMs
	stats.LayoutDirCount += other.LayoutDirCount
	stats.ValuesDirCount += other.ValuesDirCount
	stats.DrawableDirCount += other.DrawableDirCount
	if other.MaxLayoutScanMs > stats.MaxLayoutScanMs {
		stats.MaxLayoutScanMs = other.MaxLayoutScanMs
	}
	if other.MaxValuesScanMs > stats.MaxValuesScanMs {
		stats.MaxValuesScanMs = other.MaxValuesScanMs
	}
	if other.MaxDrawableScanMs > stats.MaxDrawableScanMs {
		stats.MaxDrawableScanMs = other.MaxDrawableScanMs
	}
}

// ScanResourceDir scans an Android res/ directory and returns a ResourceIndex.
func ScanResourceDir(resDir string) (*ResourceIndex, error) {
	idx, _, err := ScanResourceDirWithStats(resDir)
	return idx, err
}

// ScanResourceDirWithStats scans an Android res/ directory and returns the
// resource index plus coarse timing for the scan internals.
func ScanResourceDirWithStats(resDir string) (*ResourceIndex, ResourceScanStats, error) {
	return ScanResourceDirWithStatsWorkers(resDir, runtime.NumCPU())
}

// ScanResourceDirWithStatsWorkers scans an Android res/ directory with an
// explicit worker cap and returns the resource index plus coarse timing.
func ScanResourceDirWithStatsWorkers(resDir string, maxWorkers int) (*ResourceIndex, ResourceScanStats, error) {
	return scanResourceDirWithStatsWorkers(resDir, maxWorkers, true, true, true, ValuesScanAll)
}

// ScanLayoutResourcesWithStatsWorkers scans only layout and drawable-like resource
// directories from an Android res/ directory.
func ScanLayoutResourcesWithStatsWorkers(resDir string, maxWorkers int) (*ResourceIndex, ResourceScanStats, error) {
	return scanResourceDirWithStatsWorkers(resDir, maxWorkers, true, false, true, ValuesScanAll)
}

// ScanValuesResourcesWithStatsWorkers scans only values resource directories from
// an Android res/ directory.
func ScanValuesResourcesWithStatsWorkers(resDir string, maxWorkers int) (*ResourceIndex, ResourceScanStats, error) {
	return scanResourceDirWithStatsWorkers(resDir, maxWorkers, false, true, false, ValuesScanAll)
}

func ScanValuesResourcesWithStatsKindsWorkers(resDir string, maxWorkers int, kinds ValuesScanKind) (*ResourceIndex, ResourceScanStats, error) {
	return scanResourceDirWithStatsWorkers(resDir, maxWorkers, false, true, false, kinds)
}

func scanResourceDirWithStatsWorkers(resDir string, maxWorkers int, includeLayouts, includeValues, includeDrawables bool, valueKinds ValuesScanKind) (*ResourceIndex, ResourceScanStats, error) {
	idx := newResourceIndex()
	var stats ResourceScanStats

	if abs, err := filepath.Abs(resDir); err == nil {
		resDir = abs
	}

	entries, err := readResourceDirEntries(resDir)
	if err != nil {
		return nil, stats, err
	}

	jobs := buildResourceJobs(entries, includeLayouts, includeValues, includeDrawables)
	if len(jobs) == 0 {
		return idx, stats, nil
	}

	workers := clampWorkers(maxWorkers, len(jobs))
	results := make([]resourceDirResult, len(jobs))
	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(workers)
	for jobIdx, job := range jobs {
		resultIdx, order, entry := jobIdx, job.order, entries[job.order]
		g.Go(func() error {
			results[resultIdx] = processResourceDirJob(resDir, entry, order, maxWorkers, includeLayouts, includeValues, includeDrawables, valueKinds)
			return nil
		})
	}
	_ = g.Wait()

	for _, result := range results {
		if result.err != nil {
			return nil, stats, result.err
		}
		mergeStart := time.Now()
		idx.mergeFrom(result.index)
		stats.MergeMs += time.Since(mergeStart).Milliseconds()
		stats.add(result.stats)
	}

	return idx, stats, nil
}

func readResourceDirEntries(resDir string) ([]os.DirEntry, error) {
	info, err := os.Stat(resDir)
	if err != nil {
		return nil, fmt.Errorf("cannot access res directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", resDir)
	}
	entries, err := os.ReadDir(resDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read res directory: %w", err)
	}
	return entries, nil
}

func buildResourceJobs(entries []os.DirEntry, includeLayouts, includeValues, includeDrawables bool) []resourceDirResult {
	var jobs []resourceDirResult
	for i, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		isLayout := strings.HasPrefix(dirName, "layout")
		isValues := strings.HasPrefix(dirName, "values")
		isDrawable := strings.HasPrefix(dirName, "drawable") || strings.HasPrefix(dirName, "mipmap")
		if (!includeLayouts || !isLayout) &&
			(!includeValues || !isValues) &&
			(!includeDrawables || !isDrawable) {
			continue
		}
		jobs = append(jobs, resourceDirResult{order: i})
	}
	return jobs
}

func processResourceDirJob(resDir string, entry os.DirEntry, order int, maxWorkers int, includeLayouts, includeValues, includeDrawables bool, valueKinds ValuesScanKind) resourceDirResult {
	dirName := entry.Name()
	dirPath := filepath.Join(resDir, dirName)
	partial := newResourceIndex()
	var partialStats ResourceScanStats

	switch {
	case includeLayouts && strings.HasPrefix(dirName, "layout"):
		start := time.Now()
		err := partial.scanLayoutDir(dirPath, maxWorkers)
		partialStats.LayoutScanMs = time.Since(start).Milliseconds()
		partialStats.MaxLayoutScanMs = partialStats.LayoutScanMs
		partialStats.LayoutDirCount = 1
		return resourceDirResult{order: order, index: partial, stats: partialStats, err: err}
	case includeValues && strings.HasPrefix(dirName, "values"):
		start := time.Now()
		valuesStats, err := partial.scanValuesDirKinds(dirPath, maxWorkers, valueKinds)
		partialStats.add(valuesStats)
		partialStats.ValuesScanMs = time.Since(start).Milliseconds()
		partialStats.MaxValuesScanMs = partialStats.ValuesScanMs
		partialStats.ValuesDirCount = 1
		return resourceDirResult{order: order, index: partial, stats: partialStats, err: err}
	case includeDrawables && (strings.HasPrefix(dirName, "drawable") || strings.HasPrefix(dirName, "mipmap")):
		start := time.Now()
		partial.scanDrawableDir(dirPath, maxWorkers)
		partialStats.DrawableScanMs = time.Since(start).Milliseconds()
		partialStats.MaxDrawableScanMs = partialStats.DrawableScanMs
		partialStats.DrawableDirCount = 1
		return resourceDirResult{order: order, index: partial, stats: partialStats}
	}
	return resourceDirResult{order: order, index: partial}
}

// scanLayoutDir parses all XML files in a layout directory.
func (idx *ResourceIndex) scanLayoutDir(dir string, maxWorkers int) error {
	files, err := readLayoutDirFiles(dir, maxWorkers)
	if err != nil {
		return fmt.Errorf("cannot read layout dir %s: %w", dir, err)
	}
	if len(files) == 0 {
		return nil
	}

	cache := ActiveResourceIndexCache()
	var fingerprint string
	if cache != nil {
		fingerprint = computeLayoutDirFingerprint(dir, files)
		if cached, ok := cache.Load(fingerprint, ValuesScanNone, dir); ok {
			idx.mergeFrom(cached)
			return nil
		}
	}

	// Extract config qualifier from directory name (e.g. "layout-land" -> "land")
	base := filepath.Base(dir)
	qualifier := ""
	if i := strings.Index(base, "-"); i >= 0 {
		qualifier = base[i+1:]
	}

	type layoutResult struct {
		layout *Layout
		err    error
	}

	workers := clampWorkers(maxWorkers, len(files))

	results := make([]layoutResult, len(files))
	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(workers)
	for i, input := range files {
		i, input := i, input
		g.Go(func() error {
			layout, err := parseLayoutBytes(input.content)
			results[i] = layoutResult{layout: layout, err: err}
			return nil
		})
	}
	_ = g.Wait()

	dirIdx := newResourceIndex()
	for i, result := range results {
		if result.err != nil {
			return fmt.Errorf("parsing layout %s: %w", files[i].path, result.err)
		}
		layout := result.layout
		input := files[i]
		layout.Name = input.name
		layout.FilePath = input.path
		dirIdx.Layouts[input.name] = layout

		// Store in LayoutConfigs for multi-config tracking
		if dirIdx.LayoutConfigs[input.name] == nil {
			dirIdx.LayoutConfigs[input.name] = make(map[string]*Layout)
		}
		dirIdx.LayoutConfigs[input.name][qualifier] = layout

		// Collect IDs from the layout
		collectIDs(layout.RootView, dirIdx.IDs)
	}
	idx.mergeFrom(dirIdx)
	if cache != nil {
		_ = cache.SaveAsync(fingerprint, ValuesScanNone, dir, dirIdx)
	}
	return nil
}

// collectIDs walks the view tree and adds IDs to the set.
func collectIDs(v *View, ids map[string]bool) {
	if v == nil {
		return
	}
	if v.ID != "" {
		// Strip @+id/ or @id/ prefix
		id := v.ID
		id = strings.TrimPrefix(id, "@+id/")
		id = strings.TrimPrefix(id, "@id/")
		ids[id] = true
	}
	for _, child := range v.Children {
		collectIDs(child, ids)
	}
}

// scanValuesDir parses all XML files in a values directory. Single-threaded
// — callers that need a worker pool use NewLazyValuesScan / scanValuesDirKinds
// directly.
func (idx *ResourceIndex) scanValuesDir(dir string) (ResourceScanStats, error) {
	return idx.scanValuesDirKinds(dir, 1, ValuesScanAll)
}

func (idx *ResourceIndex) scanValuesDirKinds(dir string, maxWorkers int, kinds ValuesScanKind) (ResourceScanStats, error) {
	return NewLazyValuesScan(dir, maxWorkers, kinds).LoadInto(idx)
}

func scanValuesDirIndexKinds(dir string, maxWorkers int, kinds ValuesScanKind) (*ResourceIndex, ResourceScanStats, error) {
	readStart := time.Now()
	files, err := readValuesDirFiles(dir, maxWorkers)
	if err != nil {
		return nil, ResourceScanStats{}, err
	}
	if len(files) == 0 {
		return newResourceIndex(), ResourceScanStats{}, nil
	}
	readMs := time.Since(readStart).Milliseconds()

	cache := ActiveResourceIndexCache()
	var fingerprint string
	if cache != nil {
		fingerprint = computeValuesDirFingerprint(dir, kinds, files)
		if cached, ok := cache.Load(fingerprint, kinds, dir); ok {
			return cached, ResourceScanStats{ValuesReadMs: readMs}, nil
		}
	}

	type valuesResult struct {
		index *ResourceIndex
		stats ResourceScanStats
		err   error
	}

	workers := clampWorkers(maxWorkers, len(files))

	results := make([]valuesResult, len(files))
	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(workers)
	for i, input := range files {
		i, input := i, input
		g.Go(func() error {
			tmp := newResourceIndex()
			stats, err := tmp.parseValuesBytesKinds(input.path, input.content, kinds)
			results[i] = valuesResult{index: tmp, stats: stats, err: err}
			return nil
		})
	}
	_ = g.Wait()

	idx := newResourceIndex()
	var total ResourceScanStats
	total.ValuesReadMs += readMs
	for i, result := range results {
		if result.err != nil {
			return nil, total, fmt.Errorf("parsing values %s: %w", files[i].path, result.err)
		}
		idx.mergeFrom(result.index)
		total.add(result.stats)
	}
	if cache != nil {
		_ = cache.SaveAsync(fingerprint, kinds, dir, idx)
	}
	return idx, total, nil
}

// parseValuesBytesKinds is parseValuesFileKinds but reuses
// already-read bytes, so the cache fingerprint path does not force a
// second os.ReadFile on a miss.
func (idx *ResourceIndex) parseValuesBytesKinds(path string, data []byte, kinds ValuesScanKind) (ResourceScanStats, error) {
	var stats ResourceScanStats
	start := time.Now()
	if err := idx.parseValuesXMLKinds(path, data, kinds); err != nil {
		return stats, err
	}
	stats.ValuesParseMs += time.Since(start).Milliseconds()
	return stats, nil
}

// scanDrawableDir records drawable resource names from a drawable directory.
func (idx *ResourceIndex) scanDrawableDir(dir string, maxWorkers int) {
	files, err := readDrawableDirFiles(dir, maxWorkers)
	if err != nil {
		return
	}
	if len(files) == 0 {
		return
	}

	cache := ActiveResourceIndexCache()
	var fingerprint string
	if cache != nil {
		fingerprint = computeDrawableDirFingerprint(dir, files)
		if cached, ok := cache.Load(fingerprint, ValuesScanNone, dir); ok {
			idx.mergeFrom(cached)
			return
		}
	}

	dirIdx := newResourceIndex()
	for _, f := range files {
		dirIdx.Drawables = append(dirIdx.Drawables, f.name)
		if f.ext == ".xml" {
			dirIdx.parseDrawableSelectorBytes(f.path, f.name, f.content)
		}
	}
	idx.mergeFrom(dirIdx)
	if cache != nil {
		_ = cache.SaveAsync(fingerprint, ValuesScanNone, dir, dirIdx)
	}
}

// isSelectorStateAttr reports whether a `<selector><item>` attribute name
// behaves as a state qualifier. An item matches when the runtime's current
// state set is a superset of the item's qualifiers; items with no qualifiers
// match everything. Android-namespaced state attrs (`android:state_*`) and
// any non-android-namespaced attr (which may declare a custom state — see
// https://issuetracker.google.com/22339) count as qualifiers; bare
// presentation attrs like `android:drawable` do not.
func isSelectorStateAttr(name string) bool {
	if strings.HasPrefix(name, "android:state_") {
		return true
	}
	if i := strings.IndexByte(name, ':'); i > 0 {
		return name[:i] != "android"
	}
	return false
}

func (idx *ResourceIndex) parseDrawableSelectorBytes(path, resName string, data []byte) {
	root, err := ParseXMLAST(context.Background(), data)
	if err != nil || root == nil || root.Tag != "selector" {
		return
	}
	var items []SelectorItem
	for _, child := range root.Children {
		if child == nil || child.Tag != "item" {
			continue
		}
		item := SelectorItem{
			FilePath:   path,
			Line:       child.Line,
			StateAttrs: make(map[string]string),
		}
		for _, attr := range child.Attrs {
			if isSelectorStateAttr(attr.Name) {
				item.StateAttrs[attr.Name] = attr.Value
			}
		}
		items = append(items, item)
	}
	if len(items) > 0 {
		if idx.DrawableSelectors == nil {
			idx.DrawableSelectors = make(map[string][]SelectorItem)
		}
		idx.DrawableSelectors[resName] = append(idx.DrawableSelectors[resName], items...)
	}
}

func parseLayoutBytes(data []byte) (*Layout, error) {
	rootNode, err := ParseXMLAST(context.Background(), data)
	if err != nil {
		return nil, err
	}
	root := viewFromXMLNode(rootNode)

	return &Layout{RootView: root}, nil
}

func viewFromXMLNode(node *XMLNode) *View {
	if node == nil {
		return nil
	}
	v := &View{
		Type:       node.Tag,
		Attributes: make(map[string]string),
		Line:       node.Line,
	}

	for _, attr := range node.Attrs {
		v.Attributes[attr.Name] = attr.Value
		switch attr.Name {
		case "android:id":
			v.ID = attr.Value
		case "android:contentDescription":
			v.ContentDescription = attr.Value
		case "android:text":
			v.Text = attr.Value
		case "android:layout_width":
			v.LayoutWidth = attr.Value
		case "android:layout_height":
			v.LayoutHeight = attr.Value
		case "android:background":
			v.Background = attr.Value
		}
	}

	for _, child := range node.Children {
		if viewChild := viewFromXMLNode(child); viewChild != nil {
			v.Children = append(v.Children, viewChild)
		}
	}
	return v
}

func (idx *ResourceIndex) parseValuesXMLKinds(path string, data []byte, kinds ValuesScanKind) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	needStringLines := kinds&ValuesScanStrings != 0
	needExtraTextLines := kinds&ValuesScanExtraText != 0
	needDimenLines := kinds&ValuesScanDimensions != 0
	var lines *scanner.File
	if needStringLines || needExtraTextLines || needDimenLines {
		lines = &scanner.File{Content: data, Language: scanner.LangXML, Path: path}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("parsing XML %s: %w", path, err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "resources" {
			return fmt.Errorf("parsing XML %s: root element is not <resources>", path)
		}
		return idx.parseResourcesRootKinds(dec, path, data, start, kinds, lines)
	}
	return fmt.Errorf("parsing XML %s: root element is not <resources>", path)
}

func (idx *ResourceIndex) parseResourcesRootKinds(dec *xml.Decoder, path string, _ []byte, root xml.StartElement, kinds ValuesScanKind, lines *scanner.File) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("parsing XML %s: %w", path, err)
		}

		switch t := tok.(type) {
		case xml.CharData:
			if kinds&ValuesScanExtraText != 0 {
				raw := string(t)
				text := strings.TrimSpace(raw)
				if text != "" {
					offset := dec.InputOffset() - int64(len(t)) + int64(leadingTrimmedBytes(raw))
					idx.ExtraTexts = append(idx.ExtraTexts, ExtraTextEntry{
						FilePath: path,
						Line:     lineAtOffset(lines, offset),
						Text:     text,
					})
				}
			}
		case xml.StartElement:
			elemLine := 0
			if lines != nil {
				switch t.Name.Local {
				case "string", "dimen", "style":
					elemLine = lineAtOffset(lines, dec.InputOffset())
				}
			}
			if err := idx.parseResourceElementKinds(dec, t, path, elemLine, kinds, lines); err != nil {
				return fmt.Errorf("parsing XML %s: %w", path, err)
			}
		case xml.EndElement:
			if t.Name.Local == root.Name.Local {
				return nil
			}
		}
	}
}

func (idx *ResourceIndex) parseResourceElementKinds(dec *xml.Decoder, start xml.StartElement, path string, line int, kinds ValuesScanKind, lines *scanner.File) error {
	switch start.Name.Local {
	case "string":
		return idx.parseStringElement(dec, start, path, line, kinds)
	case "color":
		return idx.parseColorElement(dec, start, kinds)
	case "dimen":
		return idx.parseDimenElement(dec, start, path, line, kinds)
	case "integer":
		return idx.parseIntegerElement(dec, start, kinds)
	case "bool":
		return idx.parseBoolElement(dec, start, kinds)
	case "style":
		if kinds&ValuesScanDimensions == 0 {
			return skipElement(dec, start)
		}
		return idx.parseStyleElement(dec, start, path, line, lines)
	case "string-array":
		if kinds&ValuesScanArrays == 0 {
			return skipElement(dec, start)
		}
		return idx.parseStringArrayElement(dec, start)
	case "plurals":
		if kinds&ValuesScanPlurals == 0 {
			return skipElement(dec, start)
		}
		return idx.parsePluralsElement(dec, start)
	default:
		return skipElement(dec, start)
	}
}

func (idx *ResourceIndex) parseStringElement(dec *xml.Decoder, start xml.StartElement, path string, line int, kinds ValuesScanKind) error {
	if kinds&ValuesScanStrings == 0 {
		return skipElement(dec, start)
	}
	name := xmlAttr(start.Attr, "name")
	translatable := xmlAttr(start.Attr, "translatable")
	formatted := xmlAttr(start.Attr, "formatted")
	raw, text, err := decodeSimpleElementTextWithRaw(dec, start)
	if err != nil {
		return err
	}
	if name == "" {
		return nil
	}
	idx.Strings[name] = text
	if translatable == "false" {
		idx.StringsNonTranslate[name] = true
	}
	if formatted == "false" {
		if idx.StringsNonFormatted == nil {
			idx.StringsNonFormatted = make(map[string]bool)
		}
		idx.StringsNonFormatted[name] = true
	}
	if hasTrailingWhitespace(raw) {
		if idx.StringsTrailingWS == nil {
			idx.StringsTrailingWS = make(map[string]bool)
		}
		idx.StringsTrailingWS[name] = true
	}
	idx.StringsLocation[name] = StringLocation{FilePath: path, Line: line}
	return nil
}

func (idx *ResourceIndex) parseColorElement(dec *xml.Decoder, start xml.StartElement, kinds ValuesScanKind) error {
	if kinds&ValuesScanStrings == 0 {
		return skipElement(dec, start)
	}
	name := xmlAttr(start.Attr, "name")
	text, err := decodeSimpleElementText(dec, start)
	if err != nil {
		return err
	}
	if name != "" {
		idx.Colors[name] = text
	}
	return nil
}

func (idx *ResourceIndex) parseDimenElement(dec *xml.Decoder, start xml.StartElement, path string, line int, kinds ValuesScanKind) error {
	if kinds&ValuesScanDimensions == 0 {
		return skipElement(dec, start)
	}
	name := xmlAttr(start.Attr, "name")
	text, err := decodeSimpleElementText(dec, start)
	if err != nil {
		return err
	}
	if name != "" {
		idx.Dimensions[name] = text
		if idx.DimensionsLocation == nil {
			idx.DimensionsLocation = make(map[string]StringLocation)
		}
		idx.DimensionsLocation[name] = StringLocation{FilePath: path, Line: line}
	}
	return nil
}

func (idx *ResourceIndex) parseIntegerElement(dec *xml.Decoder, start xml.StartElement, kinds ValuesScanKind) error {
	if kinds&ValuesScanDimensions == 0 {
		return skipElement(dec, start)
	}
	name := xmlAttr(start.Attr, "name")
	text, err := decodeSimpleElementText(dec, start)
	if err != nil {
		return err
	}
	if name != "" {
		idx.Integers[name] = text
	}
	return nil
}

func (idx *ResourceIndex) parseBoolElement(dec *xml.Decoder, start xml.StartElement, kinds ValuesScanKind) error {
	if kinds&ValuesScanDimensions == 0 {
		return skipElement(dec, start)
	}
	name := xmlAttr(start.Attr, "name")
	text, err := decodeSimpleElementText(dec, start)
	if err != nil {
		return err
	}
	if name != "" {
		idx.Booleans[name] = text
	}
	return nil
}

func (idx *ResourceIndex) parseStyleElement(dec *xml.Decoder, start xml.StartElement, path string, line int, lines *scanner.File) error {
	name := xmlAttr(start.Attr, "name")
	if name == "" {
		return skipElement(dec, start)
	}

	style := &Style{
		Name:      name,
		Parent:    xmlAttr(start.Attr, "parent"),
		FilePath:  path,
		Line:      line,
		Items:     make(map[string]string),
		ItemLines: make(map[string]int),
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local != "item" {
				if err := skipElement(dec, t); err != nil {
					return err
				}
				continue
			}
			itemName := xmlAttr(t.Attr, "name")
			itemLine := 0
			if lines != nil {
				itemLine = lineAtOffset(lines, dec.InputOffset())
			}
			text, err := decodeSimpleElementText(dec, t)
			if err != nil {
				return err
			}
			if itemName != "" {
				style.Items[itemName] = text
				style.ItemLines[itemName] = itemLine
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				idx.Styles[name] = style
				return nil
			}
		}
	}
}

func (idx *ResourceIndex) parseStringArrayElement(dec *xml.Decoder, start xml.StartElement) error {
	name := xmlAttr(start.Attr, "name")
	var items []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local != "item" {
				if err := skipElement(dec, t); err != nil {
					return err
				}
				continue
			}
			text, err := decodeSimpleElementText(dec, t)
			if err != nil {
				return err
			}
			items = append(items, text)
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				if name != "" {
					idx.StringArrays[name] = items
				}
				return nil
			}
		}
	}
}

func (idx *ResourceIndex) parsePluralsElement(dec *xml.Decoder, start xml.StartElement) error {
	name := xmlAttr(start.Attr, "name")
	quantities := make(map[string]string)
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local != "item" {
				if err := skipElement(dec, t); err != nil {
					return err
				}
				continue
			}
			quantity := xmlAttr(t.Attr, "quantity")
			text, err := decodeSimpleElementText(dec, t)
			if err != nil {
				return err
			}
			if quantity != "" {
				quantities[quantity] = text
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				if name != "" {
					idx.Plurals[name] = quantities
				}
				return nil
			}
		}
	}
}

func decodeSimpleElementText(dec *xml.Decoder, start xml.StartElement) (string, error) {
	_, text, err := decodeSimpleElementTextWithRaw(dec, start)
	return text, err
}

func decodeSimpleElementTextWithRaw(dec *xml.Decoder, start xml.StartElement) (string, string, error) {
	var builder strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			builder.Write([]byte(t))
		case xml.StartElement:
			if err := skipElement(dec, t); err != nil {
				return "", "", err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				raw := builder.String()
				return raw, strings.TrimSpace(raw), nil
			}
		}
	}
}

func hasTrailingWhitespace(s string) bool {
	trimmed := strings.TrimRight(s, " \t\n\r")
	return trimmed != "" && trimmed != s
}

func skipElement(dec *xml.Decoder, _ xml.StartElement) error {
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

func xmlAttr(attrs []xml.Attr, name string) string {
	for _, attr := range attrs {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func leadingTrimmedBytes(s string) int {
	count := 0
	for _, r := range s {
		if !isXMLWhitespace(r) {
			break
		}
		count += len(string(r))
	}
	return count
}

func isXMLWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

// ViewCount returns the total number of views in a layout.
func (l *Layout) ViewCount() int {
	if l.RootView == nil {
		return 0
	}
	return countViews(l.RootView)
}

func countViews(v *View) int {
	count := 1
	for _, child := range v.Children {
		count += countViews(child)
	}
	return count
}

// MaxDepth returns the maximum nesting depth of the layout.
func (l *Layout) MaxDepth() int {
	if l.RootView == nil {
		return 0
	}
	return viewDepth(l.RootView)
}

func viewDepth(v *View) int {
	maxVal := 1
	for _, child := range v.Children {
		d := 1 + viewDepth(child)
		if d > maxVal {
			maxVal = d
		}
	}
	return maxVal
}

// FindViewsByType returns all views of the given type in the layout tree.
func (l *Layout) FindViewsByType(viewType string) []*View {
	if l.RootView == nil {
		return nil
	}
	var result []*View
	findByType(l.RootView, viewType, &result)
	return result
}

func findByType(v *View, viewType string, result *[]*View) {
	if v.Type == viewType {
		*result = append(*result, v)
	}
	for _, child := range v.Children {
		findByType(child, viewType, result)
	}
}

// HasHardcodedText checks if a view has hardcoded text (not a resource reference).
func (v *View) HasHardcodedText() bool {
	if v.Text == "" {
		return false
	}
	return !strings.HasPrefix(v.Text, "@string/") && !strings.HasPrefix(v.Text, "@android:string/")
}

// HasContentDescription checks if a view has a contentDescription attribute set.
func (v *View) HasContentDescription() bool {
	return v.ContentDescription != ""
}

// IsScrollableView returns true if the view type is a scrolling container.
func IsScrollableView(viewType string) bool {
	switch viewType {
	case "ScrollView", "HorizontalScrollView", "NestedScrollView",
		"androidx.core.widget.NestedScrollView":
		return true
	}
	return false
}

// IsLayoutView returns true if the view type is a layout container.
func IsLayoutView(viewType string) bool {
	switch viewType {
	case "LinearLayout", "RelativeLayout", "FrameLayout",
		"ConstraintLayout", "CoordinatorLayout", "TableLayout",
		"GridLayout", "androidx.constraintlayout.widget.ConstraintLayout",
		"androidx.coordinatorlayout.widget.CoordinatorLayout":
		return true
	}
	return false
}
