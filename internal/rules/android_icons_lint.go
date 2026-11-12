package rules

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/scanner"
)

// iconBaseName is Lint's getBaseName: the file name up to its first dot.
func iconBaseName(fileName string) string {
	if i := strings.IndexByte(fileName, '.'); i >= 0 {
		return fileName[:i]
	}
	return fileName
}

func iconFolderOf(ic *android.IconFile) string {
	return filepath.Base(filepath.Dir(ic.Path))
}

// iconIsSizedBitmap reports whether Lint reads the pixel size of a file with this name: png (not a
// nine-patch), jpg, jpeg or webp.
func iconIsSizedBitmap(name string) bool {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".9.png"):
		return false
	case strings.HasSuffix(lower, ".png"), strings.HasSuffix(lower, ".jpg"),
		strings.HasSuffix(lower, ".jpeg"), strings.HasSuffix(lower, ".webp"):
		return true
	}
	return false
}

func isAdaptiveIconLayer(base string) bool {
	return strings.HasPrefix(base, "ic_launcher") && (strings.HasSuffix(base, "_foreground") || strings.HasSuffix(base, "_background"))
}

// iconManifestKey is how the manifest icon sets spell a resource: mipmaps carry their type prefix.
func iconManifestKey(folder, base string) string {
	if strings.HasPrefix(folder, "mipmap") {
		return "@mipmap/" + base
	}
	return base
}

// isLintLauncherIcon is Lint's isLauncherIcon: an ic_launcher* name or an icon the manifest assigns.
func isLintLauncherIcon(idx *android.IconIndex, folder, base string) bool {
	if strings.HasPrefix(base, "ic_launcher") && !isAdaptiveIconLayer(base) {
		return true
	}
	return idx.LauncherIcons[iconManifestKey(folder, base)]
}

func isLintRoundIcon(idx *android.IconIndex, folder, base string) bool {
	return strings.HasSuffix(base, "_round") || idx.RoundIcons[iconManifestKey(folder, base)]
}

// lintFormatList joins items like Lint.formatList: past limit items it appends "... (N more)".
func lintFormatList(items []string, limit int) string {
	if limit <= 0 || len(items) <= limit {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:limit], ", ") + fmt.Sprintf("... (%d more)", len(items)-limit)
}

// densityQualifiers are the density segments Lint strips from a folder name to find sibling densities.
var densityQualifiers = []string{"-ldpi", "-mdpi", "-hdpi", "-xhdpi", "-xxhdpi", "-xxxhdpi"}

// stripDensityQualifier removes the first density qualifier from a folder name.
func stripDensityQualifier(folder string) (string, bool) {
	for _, q := range densityQualifiers {
		if i := strings.Index(folder, q); i >= 0 {
			return folder[:i] + folder[i+len(q):], true
		}
	}
	return "", false
}

// mdpiScalingFactor is Lint's getMdpiScalingFactor; folders without a density qualifier give 0.
func mdpiScalingFactor(folder string) float32 {
	switch {
	case strings.Contains(folder, "-mdpi"):
		return 1
	case strings.Contains(folder, "-hdpi"):
		return 1.5
	case strings.Contains(folder, "-xhdpi"):
		return 2
	case strings.Contains(folder, "-xxhdpi"):
		return 3
	case strings.Contains(folder, "-xxxhdpi"):
		return 4
	case strings.Contains(folder, "-ldpi"):
		return 0.75
	}
	return 0
}

// javaRound is Math.round(float): ties round toward positive infinity.
func javaRound(x float32) int {
	return int(math.Floor(float64(x) + 0.5))
}

var dipNameRe = regexp.MustCompile(`^.+_(\d+)dp\.[a-zA-Z]+$`)

type dipEntry struct {
	ic   *android.IconFile
	w, h int
}

// CheckIconDipSize mirrors Lint: icons that share a name and configuration but differ by density must
// compute to about the same dp size, and a name like icon_24dp.png must match its actual dp size.
func CheckIconDipSize(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	type groupKey struct{ name, config string }
	groups := make(map[groupKey][]*android.IconFile)
	for _, ic := range idx.AllIcons() {
		name := filepath.Base(ic.Path)
		if !iconIsSizedBitmap(name) || ic.Width <= 0 || ic.Height <= 0 {
			continue
		}
		config, ok := stripDensityQualifier(iconFolderOf(ic))
		if !ok {
			continue
		}
		key := groupKey{name, config}
		groups[key] = append(groups[key], ic)
	}
	var findings []scanner.Finding
	for key, files := range groups {
		findings = append(findings, dipSizeGroupFindings(key.name, files)...)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Message < findings[j].Message
	})
	c.AppendAll(findings)
}

func dipSizeGroupFindings(name string, files []*android.IconFile) []scanner.Finding {
	var out []scanner.Finding
	var entries []dipEntry
	for _, ic := range files {
		factor := mdpiScalingFactor(iconFolderOf(ic))
		if factor <= 0 {
			continue
		}
		e := dipEntry{ic, javaRound(float32(ic.Width) / factor), javaRound(float32(ic.Height) / factor)}
		entries = append(entries, e)
		if f, ok := suspiciousDipNameFinding(name, e); ok {
			out = append(out, f)
		}
	}
	if f, ok := dipVarianceFinding(name, entries); ok {
		out = append(out, f)
	}
	return out
}

// suspiciousDipNameFinding flags a name that implies a dp size (icon_24dp.png) the file does not have.
func suspiciousDipNameFinding(name string, e dipEntry) (scanner.Finding, bool) {
	m := dipNameRe.FindStringSubmatch(name)
	if m == nil {
		return scanner.Finding{}, false
	}
	implied, err := strconv.Atoi(m[1])
	if err != nil {
		return scanner.Finding{}, false
	}
	if absInt(e.w-implied) <= 2 && absInt(e.h-implied) <= 2 {
		return scanner.Finding{}, false
	}
	return iconFinding("IconDipSize", e.ic.Path, fmt.Sprintf(
		"Suspicious file name `%s`: The implied %s `dp` size does not match the actual `dp` size (pixel size %d×%d in a `%s` folder computes to %d×%d `dp`)",
		name, m[1], e.ic.Width, e.ic.Height, iconFolderOf(e.ic), e.w, e.h)), true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// dipVarianceFinding reports the density variants of one icon when their dp sizes spread by more than a
// tenth of the average, at the variant with the smallest dp size.
func dipVarianceFinding(name string, entries []dipEntry) (scanner.Finding, bool) {
	n := len(entries)
	if n == 0 {
		return scanner.Finding{}, false
	}
	sumW, sumH := 0, 0
	for _, e := range entries {
		sumW += e.w
		sumH += e.h
	}
	avgW, avgH := sumW/n, sumH/n
	sqW, sqH := 0, 0
	for _, e := range entries {
		sqW += (e.w - avgW) * (e.w - avgW)
		sqH += (e.h - avgH) * (e.h - avgH)
	}
	stdW := math.Sqrt(float64(sqW) / float64(n))
	stdH := math.Sqrt(float64(sqH) / float64(n))
	if stdW <= float64(avgW/10) && stdH <= float64(avgH/10) {
		return scanner.Finding{}, false
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.w != b.w {
			return a.w > b.w
		}
		if a.h != b.h {
			return a.h > b.h
		}
		return a.ic.Path < b.ic.Path
	})
	items := make([]string, 0, n)
	for _, e := range entries {
		items = append(items, fmt.Sprintf("%s/%s: %dx%d dp (%dx%d px)", iconFolderOf(e.ic), filepath.Base(e.ic.Path), e.w, e.h, e.ic.Width, e.ic.Height))
	}
	sort.Strings(items)
	msg := "The image `" + name + "` varies significantly in its density-independent (dip) size across the various density versions: " + strings.Join(items, ", ")
	return iconFinding("IconDipSize", entries[n-1].ic.Path, msg), true
}

func iconFinding(rule, file, message string) scanner.Finding {
	return scanner.Finding{
		File:     file,
		Line:     1,
		Col:      1,
		RuleSet:  androidRuleSet,
		Rule:     rule,
		Severity: "warning",
		Message:  message,
	}
}

// lintDensityFolderRe matches the folders Lint compares for missing densities. ldpi is excluded as in Lint.
var lintDensityFolderRe = regexp.MustCompile(`^drawable-(nodpi|xxxhdpi|xxhdpi|xhdpi|hdpi|mdpi)$`)

type iconFolder struct {
	path  string
	names map[string]bool
}

// lintDensityFolders groups drawable file names by density folder, keyed by folder name.
func lintDensityFolders(idx *android.IconIndex) map[string]*iconFolder {
	folders := make(map[string]*iconFolder)
	for _, ic := range idx.AllIcons() {
		folder := iconFolderOf(ic)
		if !lintDensityFolderRe.MatchString(folder) {
			continue
		}
		f := folders[folder]
		if f == nil {
			f = &iconFolder{path: filepath.Dir(ic.Path), names: make(map[string]bool)}
			folders[folder] = f
		}
		f.names[filepath.Base(ic.Path)] = true
	}
	return folders
}

func iconBases(names map[string]bool) map[string]bool {
	bases := make(map[string]bool, len(names))
	for n := range names {
		bases[iconBaseName(n)] = true
	}
	return bases
}

// iconNameDifferences returns the names in a whose base name no name in b shares.
func iconNameDifferences(a, b map[string]bool) []string {
	other := iconBases(b)
	var out []string
	for n := range a {
		if !other[iconBaseName(n)] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// iconNameIntersection returns the names, from both sets, whose base name appears in both.
func iconNameIntersection(a, b map[string]bool) map[string]bool {
	ba, bb := iconBases(a), iconBases(b)
	out := make(map[string]bool)
	for _, set := range []map[string]bool{a, b} {
		for n := range set {
			base := iconBaseName(n)
			if ba[base] && bb[base] {
				out[n] = true
			}
		}
	}
	return out
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedFolderNames(folders map[string]*iconFolder) []string {
	names := make([]string, 0, len(folders))
	for n := range folders {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// CheckIconDensities mirrors Lint: a required drawable density folder is missing the drawables the other
// density folders provide. Only drawable-* folders count and names match by base name.
func CheckIconDensities(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	folders := lintDensityFolders(idx)
	all := make(map[string]bool)
	for name, f := range folders {
		if strings.Contains(name, "-nodpi") {
			continue
		}
		for n := range f.names {
			all[n] = true
		}
	}
	required := make(map[string]bool, len(requiredDensityFolders))
	for _, r := range requiredDensityFolders {
		required[r] = true
	}
	var findings []scanner.Finding
	for _, name := range sortedFolderNames(folders) {
		f := folders[name]
		if strings.Contains(name, "-nodpi") || len(f.names) == len(all) || !required[name] {
			continue
		}
		diff := iconNameDifferences(all, f.names)
		if len(diff) == 0 {
			continue
		}
		msg := fmt.Sprintf("Missing the following drawables in `%s`: %s%s", name, lintFormatList(diff, 5), foundInSuffix(folders, diff))
		findings = append(findings, iconFinding("IconDensities", f.path, msg))
	}
	c.AppendAll(findings)
}

// foundInSuffix names the folders that do have the one missing drawable.
func foundInSuffix(folders map[string]*iconFolder, missing []string) string {
	if len(missing) != 1 {
		return ""
	}
	var where []string
	for name, f := range folders {
		if f.names[missing[0]] {
			where = append(where, name)
		}
	}
	if len(where) == 0 {
		return ""
	}
	sort.Strings(where)
	return " (found in " + lintFormatList(where, 5) + ")"
}

// CheckIconNoDpi reports images that appear in both drawable-nodpi and a density folder. Lint lists them
// in one finding located at the last sorted density-folder copy.
func CheckIconNoDpi(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	folders := lintDensityFolders(idx)
	nodpi := make(map[string]bool)
	for name, f := range folders {
		if strings.Contains(name, "-nodpi") {
			for n := range f.names {
				nodpi[n] = true
			}
		}
	}
	if len(nodpi) == 0 {
		return
	}
	shared := make(map[string]bool)
	var chain []string
	for _, name := range sortedFolderNames(folders) {
		if strings.Contains(name, "-nodpi") {
			continue
		}
		for n := range iconNameIntersection(nodpi, folders[name].names) {
			shared[n] = true
			chain = append(chain, filepath.Join(folders[name].path, n))
		}
	}
	if len(shared) == 0 {
		return
	}
	c.Append(iconFinding("IconNoDpi", primaryChainFile(chain),
		"The following images appear in both `-nodpi` and in a density folder: "+lintFormatList(sortedKeys(shared), 10)))
}

// iconDensityDpi maps the density segment of a folder name to its dpi value, as Lint's Density enum does.
var iconDensityDpi = map[string]int{
	"ldpi": 120, "mdpi": 160, "tvdpi": 213, "hdpi": 240, "xhdpi": 320, "xxhdpi": 480, "xxxhdpi": 640,
	"anydpi": 65534, "nodpi": 65535,
}

func folderDensityDpi(folder string) (int, bool) {
	segments := strings.Split(folder, "-")
	for _, seg := range segments[1:] {
		if dpi, ok := iconDensityDpi[seg]; ok {
			return dpi, true
		}
		if digits, ok := strings.CutSuffix(seg, "dpi"); ok {
			if dpi, err := strconv.Atoi(digits); err == nil {
				return dpi, true
			}
		}
	}
	return 0, false
}

// expectedIconPixels is Lint's expected pixel size for an icon that is dp wide in the given folder.
func expectedIconPixels(folder string, dp int) (int, int, bool) {
	var px int
	switch {
	case strings.HasPrefix(folder, "drawable-mdpi"):
		px = dp
	case strings.HasPrefix(folder, "drawable-hdpi"):
		px = (dp*3 + 1) / 2
	case strings.HasPrefix(folder, "drawable-xhdpi"):
		px = dp * 2
	case strings.HasPrefix(folder, "drawable-xxhdpi"):
		px = dp * 3
	case strings.HasPrefix(folder, "drawable-ldpi"):
		px = int(math.Floor(float64(dp*3)/4 + 0.5))
	default:
		dpi, ok := folderDensityDpi(folder)
		if !ok {
			return 0, 0, false
		}
		px = dp * dpi / 160
	}
	return px, px, true
}

// expectedIconDp is the dp size Lint expects for an icon by its role, or 0 for icons it does not size.
func expectedIconDp(idx *android.IconIndex, folder, fileName string) int {
	base := iconBaseName(fileName)
	switch {
	case isLintLauncherIcon(idx, folder, base):
		return 48
	case strings.HasPrefix(base, "ic_action_"), strings.HasPrefix(fileName, "ic_dialog_"),
		strings.HasPrefix(fileName, "ic_tab_"):
		return 32
	case strings.HasPrefix(base, "ic_stat_"):
		return 24
	case isAdaptiveIconLayer(base):
		return 108
	case strings.HasPrefix(fileName, "ic_menu_"):
		return 32
	}
	return 0
}

// CheckIconExpectedSize mirrors Lint: launcher, action bar, dialog, tab, notification, menu and adaptive
// layer icons must be the standard size for their density folder.
func CheckIconExpectedSize(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	var findings []scanner.Finding
	for _, ic := range idx.AllIcons() {
		name := filepath.Base(ic.Path)
		if !iconIsSizedBitmap(name) || ic.Width <= 0 || ic.Height <= 0 {
			continue
		}
		folder := iconFolderOf(ic)
		dp := expectedIconDp(idx, folder, name)
		if dp == 0 {
			continue
		}
		w, h, ok := expectedIconPixels(folder, dp)
		if !ok || (ic.Width == w && ic.Height == h) {
			continue
		}
		f := iconFinding("IconExpectedSize", ic.Path, fmt.Sprintf("Incorrect icon size for `%s/%s`: expected %dx%d, but was %dx%d",
			folder, name, w, h, ic.Width, ic.Height))
		f.BinaryFix = &scanner.BinaryFix{
			Type:        scanner.BinaryFixCreateFile,
			SourcePath:  ic.Path,
			TargetPath:  ic.Path,
			Description: fmt.Sprintf("Resize icon to %dx%d (cannot auto-resize without quality loss)", w, h),
			HintOnly:    true,
		}
		findings = append(findings, f)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].File < findings[j].File })
	c.AppendAll(findings)
}

// CheckConvertToWebp mirrors Lint: when minSdk is at least 18 it reports once per res directory, at the
// largest PNG or JPEG that is not an adaptive icon layer.
func CheckConvertToWebp(idx *android.IconIndex, c *scanner.FindingCollector, facts *librarymodel.Facts) {
	if idx == nil {
		return
	}
	minSdk := idx.ManifestMinSdk
	if gradleMin, ok := iconProjectMinSdk(facts); ok {
		minSdk = gradleMin
	}
	if minSdk < 18 {
		return
	}
	var best *android.IconFile
	for _, ic := range idx.AllIcons() {
		name := strings.ToLower(filepath.Base(ic.Path))
		eligible := (strings.HasSuffix(name, ".png") && !strings.HasSuffix(name, ".9.png")) ||
			strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg")
		if !eligible || isAdaptiveIconLayer(iconBaseName(filepath.Base(ic.Path))) {
			continue
		}
		if best == nil || ic.Size > best.Size || (ic.Size == best.Size && ic.Path < best.Path) {
			best = ic
		}
	}
	if best == nil || best.Size <= 0 {
		return
	}
	f := iconFinding("ConvertToWebp", best.Path,
		"One or more images in this project can be converted to the WebP format which typically results in smaller file sizes, even for lossless conversion")
	f.BinaryFix = &scanner.BinaryFix{
		Type:         scanner.BinaryFixConvertWebP,
		SourcePath:   best.Path,
		TargetPath:   "", // will generate .webp alongside
		Description:  "Convert to WebP format and remove original image",
		DeleteSource: true,
	}
	c.Append(f)
}

// iconDuplicateGroup is a set of bitmaps with identical bytes, sorted by path.
type iconDuplicateGroup struct {
	files    []*android.IconFile
	sameName bool
}

// lintDuplicateGroups mirrors Lint's checkDuplicates: bitmaps with identical bytes across every folder of
// one res directory, except groups whose names all carry a dp size (icon_24dp.png).
func lintDuplicateGroups(idx *android.IconIndex) []iconDuplicateGroup {
	byContent := make(map[string][]*android.IconFile)
	for _, ic := range idx.AllIcons() {
		name := strings.ToLower(filepath.Base(ic.Path))
		sized := strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg") ||
			strings.HasSuffix(name, ".jpeg") || strings.HasSuffix(name, ".webp")
		if !sized || ic.Hash == "" {
			continue
		}
		byContent[ic.Hash] = append(byContent[ic.Hash], ic)
	}
	var groups []iconDuplicateGroup
	for _, files := range byContent {
		if len(files) < 2 || allDipNamed(files) {
			continue
		}
		sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
		same := true
		for _, f := range files {
			same = same && filepath.Base(f.Path) == filepath.Base(files[0].Path)
		}
		groups = append(groups, iconDuplicateGroup{files: files, sameName: same})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].files[0].Path < groups[j].files[0].Path })
	return groups
}

func allDipNamed(files []*android.IconFile) bool {
	for _, f := range files {
		if !dipNameRe.MatchString(filepath.Base(f.Path)) {
			return false
		}
	}
	return true
}

// CheckIconDuplicates reports bitmaps with identical contents under different names, like Lint, at the
// last path of the group.
func CheckIconDuplicates(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	for _, g := range lintDuplicateGroups(idx) {
		if g.sameName {
			continue
		}
		names := make([]string, len(g.files))
		for i, f := range g.files {
			names[i] = filepath.Base(f.Path)
		}
		c.Append(iconFinding("IconDuplicates", g.files[len(g.files)-1].Path,
			"The following unrelated icon files have identical contents: "+strings.Join(names, ", ")))
	}
}

// CheckIconDuplicatesConfig reports one file name with identical contents in several configuration
// folders, located at the last path of the group like Lint.
func CheckIconDuplicatesConfig(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	for _, g := range lintDuplicateGroups(idx) {
		if !g.sameName {
			continue
		}
		folders := make([]string, len(g.files))
		for i, f := range g.files {
			folders[i] = iconFolderOf(f)
		}
		c.Append(iconFinding("IconDuplicatesConfig", g.files[len(g.files)-1].Path,
			fmt.Sprintf("The `%s` icon has identical contents in the following configuration folders: %s",
				filepath.Base(g.files[0].Path), strings.Join(folders, ", "))))
	}
}

// CheckGifUsage reports every file with a .gif extension in the drawable and mipmap folders.
func CheckGifUsage(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	for _, ic := range idx.AllIcons() {
		if !strings.HasSuffix(strings.ToLower(ic.Path), ".gif") {
			continue
		}
		f := iconFinding("GifUsage", ic.Path, "Using the `.gif` format for bitmaps is discouraged")
		f.BinaryFix = &scanner.BinaryFix{
			Type:         scanner.BinaryFixConvertWebP,
			SourcePath:   ic.Path,
			TargetPath:   "", // will generate .webp alongside
			Description:  "Convert GIF to WebP format",
			DeleteSource: true,
		}
		c.Append(f)
	}
}
