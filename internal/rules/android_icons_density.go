package rules

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// CheckIconDensities mirrors Lint: for each density folder, the drawables that other density folders of the
// same resource type provide but this one lacks.
func CheckIconDensities(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	type folderKey struct{ kind, folder string }
	type folderInfo struct {
		path  string
		names map[string]bool
	}
	folders := make(map[folderKey]*folderInfo)
	for _, ic := range idx.Icons {
		if !isKnownBucketDensity(ic.Density) {
			continue
		}
		folder := filepath.Base(filepath.Dir(ic.Path))
		kind, _, _ := strings.Cut(folder, "-")
		key := folderKey{kind, folder}
		if folders[key] == nil {
			folders[key] = &folderInfo{path: filepath.Dir(ic.Path), names: map[string]bool{}}
		}
		// A nine-patch x.9.png stands for x.png.
		fileName := strings.Replace(filepath.Base(ic.Path), ".9.png", ".png", 1)
		folders[key].names[fileName] = true
	}
	var findings []scanner.Finding
	for key, info := range folders {
		union := make(map[string][]string) // file name -> folders that have it
		peers := 0
		for other, otherInfo := range folders {
			if other == key || other.kind != key.kind {
				continue
			}
			peers++
			for name := range otherInfo.names {
				union[name] = append(union[name], other.folder)
			}
		}
		if peers == 0 {
			continue
		}
		var missing []string
		for name := range union {
			if !info.names[name] {
				missing = append(missing, name)
			}
		}
		if len(missing) == 0 {
			continue
		}
		sort.Strings(missing)
		list := strings.Join(missing, ", ")
		if len(missing) > 5 {
			list = strings.Join(missing[:5], ", ") + fmt.Sprintf("... (%d more)", len(missing)-5)
		} else if len(missing) == 1 {
			where := union[missing[0]]
			sort.Strings(where)
			list += " (found in " + strings.Join(where, ", ") + ")"
		}
		findings = append(findings, scanner.Finding{
			File:     info.path,
			Line:     1,
			Col:      1,
			RuleSet:  androidRuleSet,
			Rule:     "IconDensities",
			Severity: "warning",
			Message:  fmt.Sprintf("Missing the following drawables in `%s`: %s", key.folder, list),
		})
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].File < findings[j].File })
	c.AppendAll(findings)
}

// isKnownBucketDensity reports whether name is one of the standard density buckets.
func isKnownBucketDensity(name string) bool {
	for _, d := range android.KnownDensities {
		if d.Name == name {
			return true
		}
	}
	return false
}

// CheckIconDipSize reports icons whose dimensions don't match expected DPI ratios.
func CheckIconDipSize(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	byName := idx.IconsByName()
	var findings []scanner.Finding

	for name, icons := range byName {
		// Need at least 2 densities with known dimensions
		var withDims []*android.IconFile
		for _, ic := range icons {
			if ic.Width > 0 && ic.Height > 0 {
				withDims = append(withDims, ic)
			}
		}
		if len(withDims) < 2 {
			continue
		}

		// Use as reference the variant most others agree with (lowest density on ties), so one
		// wrong variant is reported instead of every correct one measured against it.
		sort.Slice(withDims, func(i, j int) bool {
			mi, mj := android.DensityMultiplier(withDims[i].Density), android.DensityMultiplier(withDims[j].Density)
			if mi != mj {
				return mi < mj
			}
			return withDims[i].Path < withDims[j].Path
		})
		consistent := func(ref, ic *android.IconFile) (bool, float64, float64) {
			refMul, mul := android.DensityMultiplier(ref.Density), android.DensityMultiplier(ic.Density)
			if refMul == 0 || mul == 0 {
				return true, 0, 0
			}
			expectedW := float64(ref.Width) / refMul * mul
			expectedH := float64(ref.Height) / refMul * mul
			// Allow 1px tolerance
			ok := math.Abs(float64(ic.Width)-expectedW) <= 1 && math.Abs(float64(ic.Height)-expectedH) <= 1
			return ok, expectedW, expectedH
		}
		var ref *android.IconFile
		best := -1
		for _, cand := range withDims {
			if android.DensityMultiplier(cand.Density) == 0 {
				continue
			}
			agree := 0
			for _, ic := range withDims {
				if ok, _, _ := consistent(cand, ic); ok {
					agree++
				}
			}
			if agree > best {
				ref, best = cand, agree
			}
		}
		if ref == nil {
			continue
		}
		for _, ic := range withDims {
			if ic == ref {
				continue
			}
			ok, _, _ := consistent(ref, ic)
			if !ok {
				findings = append(findings, scanner.Finding{
					File:     ic.Path,
					Line:     1,
					Col:      1,
					RuleSet:  androidRuleSet,
					Rule:     "IconDipSize",
					Severity: "warning",
					Message:  iconDipSizeMessage(name, withDims),
				})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].File < findings[j].File })
	c.AppendAll(findings)
}

// iconDipSizeMessage lists every density variant of an icon with its dp and pixel size, as Lint does.
func iconDipSizeMessage(name string, variants []*android.IconFile) string {
	sorted := append([]*android.IconFile(nil), variants...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	parts := make([]string, 0, len(sorted))
	for _, ic := range sorted {
		mul := android.DensityMultiplier(ic.Density)
		if mul == 0 {
			mul = 1
		}
		parts = append(parts, fmt.Sprintf("%s/%s: %dx%d dp (%dx%d px)",
			filepath.Base(filepath.Dir(ic.Path)), filepath.Base(ic.Path),
			int(math.Round(float64(ic.Width)/mul)), int(math.Round(float64(ic.Height)/mul)), ic.Width, ic.Height))
	}
	file := name
	if len(sorted) > 0 {
		file = filepath.Base(sorted[0].Path)
	}
	return fmt.Sprintf("The image `%s` varies significantly in its density-independent (dip) size across the various density versions: %s",
		file, strings.Join(parts, ", "))
}

// requiredDensityFolders are the density folders Lint expects once a resource directory has any of them.
var requiredDensityFolders = []string{"drawable-mdpi", "drawable-hdpi", "drawable-xhdpi", "drawable-xxhdpi"}

// densityFolderNames are the folder names Lint treats as density buckets.
var densityFolderNames = map[string]bool{
	"drawable-nodpi": true, "drawable-xxxhdpi": true, "drawable-xxhdpi": true,
	"drawable-xhdpi": true, "drawable-hdpi": true, "drawable-mdpi": true,
}

// CheckIconMissingDensityFolder reports, per res directory, the required density folders that are missing
// when at least one of them exists, like Lint.
func CheckIconMissingDensityFolder(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	byRes := make(map[string]map[string]bool)
	for dir := range iconFolderFiles(idx) {
		name := filepath.Base(dir)
		if !densityFolderNames[name] {
			continue
		}
		res := filepath.Dir(dir)
		if byRes[res] == nil {
			byRes[res] = make(map[string]bool)
		}
		byRes[res][name] = true
	}
	resDirs := make([]string, 0, len(byRes))
	for res := range byRes {
		resDirs = append(resDirs, res)
	}
	sort.Strings(resDirs)
	for _, res := range resDirs {
		var missing []string
		hasRequired := false
		for _, want := range requiredDensityFolders {
			if byRes[res][want] {
				hasRequired = true
			} else {
				missing = append(missing, want)
			}
		}
		if len(missing) == 0 || !hasRequired {
			continue
		}
		sort.Strings(missing)
		c.Append(scanner.Finding{
			File:     res,
			Line:     1,
			Col:      1,
			RuleSet:  androidRuleSet,
			Rule:     "IconMissingDensityFolder",
			Severity: "warning",
			Message:  fmt.Sprintf("Missing density variation folders in `%s`: %s", lintDisplayPath(res), strings.Join(missing, ", ")),
		})
	}
}

// CheckIconExpectedSize reports launcher icons that are not at the expected size for their density.
func CheckIconExpectedSize(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}

	for _, ic := range idx.Icons {
		if ic.Width == 0 || ic.Height == 0 {
			continue
		}
		if !isLauncherIconName(ic.Name) {
			continue
		}

		expected, ok := expectedLauncherSizes[ic.Density]
		if !ok {
			continue
		}

		if ic.Width != expected || ic.Height != expected {
			c.Append(scanner.Finding{
				File:     ic.Path,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "IconExpectedSize",
				Severity: "warning",
				Message: fmt.Sprintf("Incorrect icon size for `%s/%s`: expected %dx%d, but was %dx%d",
					filepath.Base(filepath.Dir(ic.Path)), filepath.Base(ic.Path), expected, expected, ic.Width, ic.Height),
				BinaryFix: &scanner.BinaryFix{
					Type:        scanner.BinaryFixCreateFile,
					SourcePath:  ic.Path,
					TargetPath:  ic.Path,
					Description: fmt.Sprintf("Resize launcher icon to %dx%d (cannot auto-resize without quality loss)", expected, expected),
					HintOnly:    true,
				},
			})
		}
	}
}

// CheckIconNoDpi reports images that appear in both drawable-nodpi and a density folder. Lint lists them in
// one finding located at the first density-specific copy.
func CheckIconNoDpi(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	nodpiNames := make(map[string]bool)
	for _, ic := range idx.Densities["nodpi"] {
		nodpiNames[filepath.Base(ic.Path)] = true
	}
	if len(nodpiNames) == 0 {
		return
	}
	shadowed := make(map[string]bool)
	var first string
	for density, icons := range idx.Densities {
		if density == "nodpi" {
			continue
		}
		for _, ic := range icons {
			name := filepath.Base(ic.Path)
			if !nodpiNames[name] {
				continue
			}
			shadowed[name] = true
			if first == "" || ic.Path < first {
				first = ic.Path
			}
		}
	}
	if len(shadowed) == 0 {
		return
	}
	names := make([]string, 0, len(shadowed))
	for n := range shadowed {
		names = append(names, n)
	}
	sort.Strings(names)
	c.Append(scanner.Finding{
		File:     first,
		Line:     1,
		Col:      1,
		RuleSet:  androidRuleSet,
		Rule:     "IconNoDpi",
		Severity: "warning",
		Message:  "The following images appear in both `-nodpi` and in a density folder: " + strings.Join(names, ", "),
	})
}

// CheckIconDuplicatesConfig reports a file name whose contents are identical in several configuration folders
// (density folders included), located at the last such folder like Lint.
func CheckIconDuplicatesConfig(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	type fileHash struct{ name, hash string }
	groups := make(map[fileHash][]*android.IconFile)
	for _, list := range [][]*android.IconFile{idx.Icons, idx.ConfigIcons} {
		for _, ic := range list {
			if ic.Hash == "" || ic.Format == "xml" {
				continue
			}
			key := fileHash{filepath.Base(ic.Path), ic.Hash}
			groups[key] = append(groups[key], ic)
		}
	}
	var findings []scanner.Finding
	for key, icons := range groups {
		folderSet := make(map[string]*android.IconFile)
		for _, ic := range icons {
			folderSet[filepath.Base(filepath.Dir(ic.Path))] = ic
		}
		if len(folderSet) < 2 {
			continue
		}
		folders := make([]string, 0, len(folderSet))
		for folder := range folderSet {
			folders = append(folders, folder)
		}
		sort.Strings(folders)
		findings = append(findings, scanner.Finding{
			File:     folderSet[folders[len(folders)-1]].Path,
			Line:     1,
			Col:      1,
			RuleSet:  androidRuleSet,
			Rule:     "IconDuplicatesConfig",
			Severity: "warning",
			Message: fmt.Sprintf("The `%s` icon has identical contents in the following configuration folders: %s",
				key.name, strings.Join(folders, ", ")),
		})
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	c.AppendAll(findings)
}
