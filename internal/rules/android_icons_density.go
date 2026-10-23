package rules

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// CheckIconDensities reports icons that exist in some densities but not others.
func CheckIconDensities(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	byName := idx.IconsByName()
	var findings []scanner.Finding

	for name, icons := range byName {
		if len(icons) < 2 {
			continue // need at least 2 densities to compare
		}
		present := make(map[string]bool)
		var samplePath string
		for _, ic := range icons {
			present[ic.Density] = true
			if samplePath == "" {
				samplePath = ic.Path
			}
		}
		// Check which standard densities are missing
		var missing []string
		for _, d := range android.KnownDensities {
			if !present[d.Name] {
				missing = append(missing, d.Name)
			}
		}
		if len(missing) > 0 && len(missing) < len(android.KnownDensities) {
			findings = append(findings, scanner.Finding{
				File:     samplePath,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "IconDensities",
				Severity: "warning",
				Message:  fmt.Sprintf("Icon '%s' is missing density variants: %s", name, strings.Join(missing, ", ")),
			})
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	c.AppendAll(findings)
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

		// Compute expected base dp size from the first icon
		ref := withDims[0]
		refMul := android.DensityMultiplier(ref.Density)
		if refMul == 0 {
			continue
		}
		baseDp := float64(ref.Width) / refMul

		for _, ic := range withDims[1:] {
			mul := android.DensityMultiplier(ic.Density)
			if mul == 0 {
				continue
			}
			expectedW := baseDp * mul
			expectedH := (float64(ref.Height) / refMul) * mul

			// Allow 1px tolerance
			if math.Abs(float64(ic.Width)-expectedW) > 1 || math.Abs(float64(ic.Height)-expectedH) > 1 {
				findings = append(findings, scanner.Finding{
					File:     ic.Path,
					Line:     1,
					Col:      1,
					RuleSet:  androidRuleSet,
					Rule:     "IconDipSize",
					Severity: "warning",
					Message: fmt.Sprintf("Icon '%s' in %s is %dx%d but expected ~%.0fx%.0f based on %s variant",
						name, ic.Density, ic.Width, ic.Height, expectedW, expectedH, ref.Density),
				})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].File < findings[j].File })
	c.AppendAll(findings)
}

// CheckIconMissingDensityFolder reports when standard density folders are missing entirely.
// It requires that at least one drawable-* or mipmap-* folder exists.
func CheckIconMissingDensityFolder(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil || len(idx.Densities) == 0 {
		return
	}

	// Required densities if any density folder exists
	required := []string{"mdpi", "hdpi", "xhdpi", "xxhdpi"}

	var samplePath string
	for _, icons := range idx.Densities {
		if len(icons) > 0 {
			samplePath = icons[0].Path
			break
		}
	}

	for _, d := range required {
		if _, exists := idx.Densities[d]; !exists {
			c.Append(scanner.Finding{
				File:     samplePath,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "IconMissingDensityFolder",
				Severity: "warning",
				Message:  fmt.Sprintf("Missing density folder for '%s'. Provide resources for all common densities.", d),
			})
		}
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
		isLauncher := false
		for _, prefix := range launcherPrefixes {
			if ic.Name == prefix || strings.HasPrefix(ic.Name, prefix+"_") {
				isLauncher = true
				break
			}
		}
		if !isLauncher {
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
				Message: fmt.Sprintf("Launcher icon '%s' in %s is %dx%d but expected %dx%d (48dp base)",
					ic.Name, ic.Density, ic.Width, ic.Height, expected, expected),
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

// CheckIconNoDpi reports icons that appear in both drawable-nodpi and a
// density-specific folder. An icon in nodpi should not also exist in a
// density-specific folder, as the density variant will shadow the nodpi one.
func CheckIconNoDpi(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	// Get nodpi icons
	nodpiIcons := idx.Densities["nodpi"]
	if len(nodpiIcons) == 0 {
		return
	}
	nodpiNames := make(map[string]*android.IconFile)
	for _, ic := range nodpiIcons {
		nodpiNames[ic.Name] = ic
	}

	// Check if any nodpi icon also exists in a density-specific folder
	var findings []scanner.Finding
	for density, icons := range idx.Densities {
		if density == "nodpi" {
			continue
		}
		for _, ic := range icons {
			if nodpiIC, ok := nodpiNames[ic.Name]; ok {
				findings = append(findings, scanner.Finding{
					File:     nodpiIC.Path,
					Line:     1,
					Col:      1,
					RuleSet:  androidRuleSet,
					Rule:     "IconNoDpi",
					Severity: "warning",
					Message: fmt.Sprintf("Icon '%s' exists in both drawable-nodpi and drawable-%s. "+
						"The density-specific variant will shadow the nodpi version.",
						ic.Name, density),
				})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	c.AppendAll(findings)
}

// CheckIconDuplicatesConfig reports identical icons across different
// configuration folders (e.g., drawable-en and drawable-fr). Icons with the
// same hash in different config folders are wasteful duplicates.
func CheckIconDuplicatesConfig(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	byName := idx.ConfigIconsByName()
	var findings []scanner.Finding

	for name, icons := range byName {
		if len(icons) < 2 {
			continue
		}
		// Group by hash
		hashMap := make(map[string][]string) // hash -> configs
		pathMap := make(map[string]string)   // hash -> first path
		for _, ic := range icons {
			if ic.Hash == "" {
				continue
			}
			hashMap[ic.Hash] = append(hashMap[ic.Hash], ic.Density)
			if _, ok := pathMap[ic.Hash]; !ok {
				pathMap[ic.Hash] = ic.Path
			}
		}
		for hash, configs := range hashMap {
			if len(configs) > 1 {
				sort.Strings(configs)
				findings = append(findings, scanner.Finding{
					File:     pathMap[hash],
					Line:     1,
					Col:      1,
					RuleSet:  androidRuleSet,
					Rule:     "IconDuplicatesConfig",
					Severity: "warning",
					Message: fmt.Sprintf("Icon '%s' is identical across configurations %s (duplicate resources)",
						name, strings.Join(configs, ", ")),
				})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	c.AppendAll(findings)
}
