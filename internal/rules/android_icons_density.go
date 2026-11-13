package rules

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

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
