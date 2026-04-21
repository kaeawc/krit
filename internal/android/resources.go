package android

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
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
	StringsLocation     map[string]StringLocation     // string name -> file path and line
	Colors              map[string]string             // color name -> value
	Dimensions          map[string]string             // dimen name -> value
	Styles              map[string]*Style             // style name -> style definition
	Drawables           []string                      // drawable resource names
	StringArrays        map[string][]string           // string-array name -> items
	Plurals             map[string]map[string]string  // plural name -> quantity -> value
	Integers            map[string]string             // integer name -> value
	Booleans            map[string]string             // bool name -> value
	IDs                 map[string]bool               // declared @+id names
	ExtraTexts          []ExtraTextEntry              // stray text nodes in values files
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
	Name   string
	Parent string
	Items  map[string]string // item name -> value
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

type lineIndex struct {
	newlines []int64
}

func newLineIndex(data []byte) *lineIndex {
	newlines := make([]int64, 0, bytes.Count(data, []byte{'\n'}))
	for i, b := range data {
		if b == '\n' {
			newlines = append(newlines, int64(i))
		}
	}
	return &lineIndex{newlines: newlines}
}

func (idx *lineIndex) lineAtOffset(offset int64) int {
	if idx == nil {
		return 1
	}
	return sort.Search(len(idx.newlines), func(i int) bool {
		return idx.newlines[i] >= offset
	}) + 1
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

func newValuesDirProvider(dir string, maxWorkers int) ResourceValueProvider {
	return NewLazyValuesScan(dir, maxWorkers, ValuesScanAll)
}

func newResourceIndex() *ResourceIndex {
	return &ResourceIndex{
		Layouts:             make(map[string]*Layout),
		LayoutConfigs:       make(map[string]map[string]*Layout),
		Strings:             make(map[string]string),
		StringsNonTranslate: make(map[string]bool),
		StringsNonFormatted: make(map[string]bool),
		StringsLocation:     make(map[string]StringLocation),
		Colors:              make(map[string]string),
		Dimensions:          make(map[string]string),
		Styles:              make(map[string]*Style),
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
	if idx.StringsLocation == nil {
		idx.StringsLocation = make(map[string]StringLocation)
	}
	if idx.Colors == nil {
		idx.Colors = make(map[string]string)
	}
	if idx.Dimensions == nil {
		idx.Dimensions = make(map[string]string)
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
	for name, loc := range other.StringsLocation {
		idx.StringsLocation[name] = loc
	}
	for name, value := range other.Colors {
		idx.Colors[name] = value
	}
	for name, value := range other.Dimensions {
		idx.Dimensions[name] = value
	}
	for name, style := range other.Styles {
		idx.Styles[name] = style
	}
	idx.Drawables = append(idx.Drawables, other.Drawables...)
	for name, items := range other.StringArrays {
		idx.StringArrays[name] = items
	}
	for name, quantities := range other.Plurals {
		idx.Plurals[name] = quantities
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
	idx.ExtraTexts = append(idx.ExtraTexts, other.ExtraTexts...)
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

	info, err := os.Stat(resDir)
	if err != nil {
		return nil, stats, fmt.Errorf("cannot access res directory: %w", err)
	}
	if !info.IsDir() {
		return nil, stats, fmt.Errorf("%s is not a directory", resDir)
	}

	entries, err := os.ReadDir(resDir)
	if err != nil {
		return nil, stats, fmt.Errorf("cannot read res directory: %w", err)
	}

	var jobs []resourceDirResult
	for i, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		isLayout := strings.HasPrefix(dirName, "layout")
		isValues := strings.HasPrefix(dirName, "values")
		isDrawable := strings.HasPrefix(dirName, "drawable") || strings.HasPrefix(dirName, "mipmap")
		if !(includeLayouts && isLayout) &&
			!(includeValues && isValues) &&
			!(includeDrawables && isDrawable) {
			continue
		}
		jobs = append(jobs, resourceDirResult{order: i})
	}
	if len(jobs) == 0 {
		return idx, stats, nil
	}

	workers := clampWorkers(maxWorkers, len(jobs))

	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	results := make([]resourceDirResult, len(jobs))
	for jobIdx, job := range jobs {
		entry := entries[job.order]
		wg.Add(1)
		sem <- struct{}{}
		go func(resultIdx int, order int, entry os.DirEntry) {
			defer wg.Done()
			defer func() { <-sem }()
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
				results[resultIdx] = resourceDirResult{order: order, index: partial, stats: partialStats, err: err}
			case includeValues && strings.HasPrefix(dirName, "values"):
				start := time.Now()
				valuesStats, err := partial.scanValuesDirKinds(dirPath, maxWorkers, valueKinds)
				partialStats.add(valuesStats)
				partialStats.ValuesScanMs = time.Since(start).Milliseconds()
				partialStats.MaxValuesScanMs = partialStats.ValuesScanMs
				partialStats.ValuesDirCount = 1
				results[resultIdx] = resourceDirResult{order: order, index: partial, stats: partialStats, err: err}
			case includeDrawables && (strings.HasPrefix(dirName, "drawable") || strings.HasPrefix(dirName, "mipmap")):
				start := time.Now()
				partial.scanDrawableDir(dirPath)
				partialStats.DrawableScanMs = time.Since(start).Milliseconds()
				partialStats.MaxDrawableScanMs = partialStats.DrawableScanMs
				partialStats.DrawableDirCount = 1
				results[resultIdx] = resourceDirResult{order: order, index: partial, stats: partialStats}
			}
		}(jobIdx, job.order, entry)
	}
	wg.Wait()

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

// scanLayoutDir parses all XML files in a layout directory.
func (idx *ResourceIndex) scanLayoutDir(dir string, maxWorkers int) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read layout dir %s: %w", dir, err)
	}

	// Extract config qualifier from directory name (e.g. "layout-land" -> "land")
	base := filepath.Base(dir)
	qualifier := ""
	if i := strings.Index(base, "-"); i >= 0 {
		qualifier = base[i+1:]
	}

	type layoutInput struct {
		name string
		path string
	}
	type layoutResult struct {
		layout *Layout
		err    error
	}

	inputs := make([]layoutInput, 0, len(files))
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".xml") {
			continue
		}
		inputs = append(inputs, layoutInput{
			name: strings.TrimSuffix(f.Name(), ".xml"),
			path: filepath.Join(dir, f.Name()),
		})
	}
	if len(inputs) == 0 {
		return nil
	}

	workers := clampWorkers(maxWorkers, len(inputs))

	results := make([]layoutResult, len(inputs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for i, input := range inputs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, input layoutInput) {
			defer wg.Done()
			defer func() { <-sem }()
			layout, err := parseLayoutFile(input.path)
			results[i] = layoutResult{layout: layout, err: err}
		}(i, input)
	}
	wg.Wait()

	for i, result := range results {
		if result.err != nil {
			return fmt.Errorf("parsing layout %s: %w", inputs[i].path, result.err)
		}
		layout := result.layout
		input := inputs[i]
		layout.Name = input.name
		layout.FilePath = input.path
		idx.Layouts[input.name] = layout

		// Store in LayoutConfigs for multi-config tracking
		if idx.LayoutConfigs[input.name] == nil {
			idx.LayoutConfigs[input.name] = make(map[string]*Layout)
		}
		idx.LayoutConfigs[input.name][qualifier] = layout

		// Collect IDs from the layout
		collectIDs(layout.RootView, idx.IDs)
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

// scanValuesDir parses all XML files in a values directory.
func (idx *ResourceIndex) scanValuesDir(dir string, maxWorkers int) (ResourceScanStats, error) {
	return idx.scanValuesDirKinds(dir, maxWorkers, ValuesScanAll)
}

func (idx *ResourceIndex) scanValuesDirKinds(dir string, maxWorkers int, kinds ValuesScanKind) (ResourceScanStats, error) {
	return NewLazyValuesScan(dir, maxWorkers, kinds).LoadInto(idx)
}

func scanValuesDirIndex(dir string, maxWorkers int) (*ResourceIndex, ResourceScanStats, error) {
	return scanValuesDirIndexKinds(dir, maxWorkers, ValuesScanAll)
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
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for i, input := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, input valuesDirFile) {
			defer wg.Done()
			defer func() { <-sem }()
			tmp := newResourceIndex()
			stats, err := tmp.parseValuesBytesKinds(input.path, input.content, kinds)
			results[i] = valuesResult{index: tmp, stats: stats, err: err}
		}(i, input)
	}
	wg.Wait()

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
		_ = cache.Save(fingerprint, kinds, dir, idx)
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
func (idx *ResourceIndex) scanDrawableDir(dir string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		// Strip extension for resource name
		ext := filepath.Ext(name)
		resName := strings.TrimSuffix(name, ext)
		idx.Drawables = append(idx.Drawables, resName)
	}
}

// parseLayoutFile parses a single layout XML file into a Layout.
func parseLayoutFile(path string) (*Layout, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	rootNode, err := ParseXMLAST(data)
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

// parseValuesFile parses a values XML file and populates the index.
func (idx *ResourceIndex) parseValuesFile(path string) (ResourceScanStats, error) {
	return idx.parseValuesFileKinds(path, ValuesScanAll)
}

func (idx *ResourceIndex) parseValuesFileKinds(path string, kinds ValuesScanKind) (ResourceScanStats, error) {
	var stats ResourceScanStats
	start := time.Now()
	data, err := os.ReadFile(path)
	if err != nil {
		return stats, err
	}
	stats.ValuesReadMs += time.Since(start).Milliseconds()

	start = time.Now()
	if err := idx.parseValuesXMLKinds(path, data, kinds); err != nil {
		return stats, err
	}
	stats.ValuesParseMs += time.Since(start).Milliseconds()
	start = time.Now()
	stats.ValuesIndexMs += time.Since(start).Milliseconds()

	return stats, nil
}

func (idx *ResourceIndex) parseValuesXML(path string, data []byte) error {
	return idx.parseValuesXMLKinds(path, data, ValuesScanAll)
}

func (idx *ResourceIndex) parseValuesXMLKinds(path string, data []byte, kinds ValuesScanKind) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	needStringLines := kinds&ValuesScanStrings != 0
	needExtraTextLines := kinds&ValuesScanExtraText != 0
	var lines *lineIndex
	if needStringLines || needExtraTextLines {
		lines = newLineIndex(data)
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
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

func (idx *ResourceIndex) parseResourcesRoot(dec *xml.Decoder, path string, data []byte, root xml.StartElement) error {
	return idx.parseResourcesRootKinds(dec, path, data, root, ValuesScanAll, newLineIndex(data))
}

func (idx *ResourceIndex) parseResourcesRootKinds(dec *xml.Decoder, path string, data []byte, root xml.StartElement, kinds ValuesScanKind, lines *lineIndex) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
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
						Line:     lines.lineAtOffset(offset),
						Text:     text,
					})
				}
			}
		case xml.StartElement:
			elemLine := 0
			if kinds&ValuesScanStrings != 0 && t.Name.Local == "string" {
				// Capture line number before decoding the element.
				elemLine = lines.lineAtOffset(dec.InputOffset())
			}
			if err := idx.parseResourceElementKinds(dec, t, path, elemLine, kinds); err != nil {
				return fmt.Errorf("parsing XML %s: %w", path, err)
			}
		case xml.EndElement:
			if t.Name.Local == root.Name.Local {
				return nil
			}
		}
	}
}

func (idx *ResourceIndex) parseResourceElement(dec *xml.Decoder, start xml.StartElement, path string, line int) error {
	return idx.parseResourceElementKinds(dec, start, path, line, ValuesScanAll)
}

func (idx *ResourceIndex) parseResourceElementKinds(dec *xml.Decoder, start xml.StartElement, path string, line int, kinds ValuesScanKind) error {
	switch start.Name.Local {
	case "string":
		if kinds&ValuesScanStrings == 0 {
			return skipElement(dec, start)
		}
		name := xmlAttr(start.Attr, "name")
		translatable := xmlAttr(start.Attr, "translatable")
		formatted := xmlAttr(start.Attr, "formatted")
		text, err := decodeSimpleElementText(dec, start)
		if err != nil {
			return err
		}
		if name != "" {
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
			idx.StringsLocation[name] = StringLocation{FilePath: path, Line: line}
		}
	case "color":
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
	case "dimen":
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
		}
	case "integer":
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
	case "bool":
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
	case "style":
		if kinds&ValuesScanDimensions == 0 {
			return skipElement(dec, start)
		}
		return idx.parseStyleElement(dec, start)
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
	return nil
}

func (idx *ResourceIndex) parseStyleElement(dec *xml.Decoder, start xml.StartElement) error {
	name := xmlAttr(start.Attr, "name")
	if name == "" {
		return skipElement(dec, start)
	}

	style := &Style{
		Name:   name,
		Parent: xmlAttr(start.Attr, "parent"),
		Items:  make(map[string]string),
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
			text, err := decodeSimpleElementText(dec, t)
			if err != nil {
				return err
			}
			if itemName != "" {
				style.Items[itemName] = text
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
	var builder strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			builder.Write([]byte(t))
		case xml.StartElement:
			if err := skipElement(dec, t); err != nil {
				return "", err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return strings.TrimSpace(builder.String()), nil
			}
		}
	}
}

func skipElement(dec *xml.Decoder, start xml.StartElement) error {
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

func lineNumberAtOffset(data []byte, offset int64) int {
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	line := 1
	for _, b := range data[:offset] {
		if b == '\n' {
			line++
		}
	}
	return line
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
	max := 1
	for _, child := range v.Children {
		d := 1 + viewDepth(child)
		if d > max {
			max = d
		}
	}
	return max
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
