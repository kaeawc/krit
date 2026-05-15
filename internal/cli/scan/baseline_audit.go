package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type baselineAuditIssue struct {
	Entry  scanner.BaselineEntry `json:"entry"`
	Reason string                `json:"reason"`
}

type baselineAuditWarning struct {
	Entry     scanner.BaselineEntry `json:"entry"`
	Stability string                `json:"stability"`
	Message   string                `json:"message"`
}

type baselineAuditReport struct {
	BaselinePath string                 `json:"baselinePath"`
	ScanPaths    []string               `json:"scanPaths"`
	StaleEntries []baselineAuditIssue   `json:"staleEntries"`
	Warnings     []baselineAuditWarning `json:"warnings,omitempty"`
}

func RunBaselineAuditColumns(columns *scanner.FindingColumns, baseline *scanner.Baseline, baselinePath, basePath string, scanPaths []string, format string) int {
	liveCapacity := 0
	if columns != nil {
		liveCapacity = columns.Len() * 2
	}
	liveIDs := make(map[string]bool, liveCapacity)
	if columns != nil {
		for i := 0; i < columns.Len(); i++ {
			liveIDs[scanner.BaselineIDAt(columns, i, "", basePath)] = true
			liveIDs[scanner.BaselineIDFilenameOnlyAt(columns, i, "")] = true
		}
	}

	knownRules := make(map[string]bool, len(api.Registry))
	ruleByID := make(map[string]*api.Rule, len(api.Registry))
	for _, rule := range api.Registry {
		knownRules[rule.ID] = true
		ruleByID[rule.ID] = rule
	}

	knownFiles := collectBaselineAuditFiles(scanPaths)
	stale, warnings := classifyBaselineEntries(baseline.Entries(), liveIDs, knownRules, ruleByID, baselinePath, basePath, knownFiles)

	sort.Slice(warnings, func(i, j int) bool {
		if warnings[i].Entry.Rule != warnings[j].Entry.Rule {
			return warnings[i].Entry.Rule < warnings[j].Entry.Rule
		}
		return warnings[i].Entry.ID < warnings[j].Entry.ID
	})

	sort.Slice(stale, func(i, j int) bool {
		if stale[i].Reason != stale[j].Reason {
			return stale[i].Reason < stale[j].Reason
		}
		if stale[i].Entry.Path != stale[j].Entry.Path {
			return stale[i].Entry.Path < stale[j].Entry.Path
		}
		if stale[i].Entry.Rule != stale[j].Entry.Rule {
			return stale[i].Entry.Rule < stale[j].Entry.Rule
		}
		return stale[i].Entry.ID < stale[j].Entry.ID
	})

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(baselineAuditReport{
			BaselinePath: baselinePath,
			ScanPaths:    append([]string(nil), scanPaths...),
			StaleEntries: stale,
			Warnings:     warnings,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "error: encode baseline-audit JSON: %v\n", err)
			return 2
		}
		return 0
	}

	fmt.Printf("Baseline audit - %s\n", baselinePath)
	if len(stale) == 0 && len(warnings) == 0 {
		fmt.Println("No stale baseline entries found.")
		return 0
	}

	if len(stale) > 0 {
		fmt.Printf("Dead baseline entries: %d\n", len(stale))
		for _, issue := range stale {
			path := issue.Entry.Path
			if path == "" {
				path = "(unknown path)"
			}
			fmt.Printf("  %s :: %s (%s)\n", path, issue.Entry.Rule, issue.Reason)
		}
	}

	if len(warnings) > 0 {
		fmt.Printf("Stability warnings: %d (rules whose output may change between minor versions)\n", len(warnings))
		for _, w := range warnings {
			path := w.Entry.Path
			if path == "" {
				path = "(unknown path)"
			}
			fmt.Printf("  %s :: %s [stability=%s]\n", path, w.Entry.Rule, w.Stability)
		}
	}
	return 0
}

// classifyBaselineEntries splits baseline entries into (stale, warnings).
// Stale entries are baseline IDs no longer produced by the scan; warnings
// flag live findings whose owning rule has StabilityEvolving so consumers
// know the pin may break on a minor-version bump.
func classifyBaselineEntries(
	entries []scanner.BaselineEntry,
	liveIDs, knownRules map[string]bool,
	ruleByID map[string]*api.Rule,
	baselinePath, basePath string,
	knownFiles map[string]bool,
) ([]baselineAuditIssue, []baselineAuditWarning) {
	stale := make([]baselineAuditIssue, 0)
	warnings := make([]baselineAuditWarning, 0)
	seenWarning := make(map[string]bool)
	for _, entry := range entries {
		if liveIDs[entry.ID] {
			if w, ok := stabilityWarning(entry, ruleByID); ok && !seenWarning[entry.ID] {
				seenWarning[entry.ID] = true
				warnings = append(warnings, w)
			}
			continue
		}
		reason := "finding no longer exists"
		switch {
		case entry.Rule == "" || !knownRules[entry.Rule]:
			reason = "rule deleted"
		case !baselineEntryFileExists(entry, baselinePath, basePath, knownFiles):
			reason = "file no longer exists"
		}
		stale = append(stale, baselineAuditIssue{Entry: entry, Reason: reason})
	}
	return stale, warnings
}

func stabilityWarning(entry scanner.BaselineEntry, ruleByID map[string]*api.Rule) (baselineAuditWarning, bool) {
	r := ruleByID[entry.Rule]
	if r == nil {
		return baselineAuditWarning{}, false
	}
	s := rules.V2RuleStability(r)
	if s != api.StabilityEvolving {
		return baselineAuditWarning{}, false
	}
	return baselineAuditWarning{
		Entry:     entry,
		Stability: s.String(),
		Message:   "rule output may change between minor versions; baseline pin is fragile",
	}, true
}

func ResolveBaselineAuditPath(explicit string, scanPaths []string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	candidates := []string{}
	seen := make(map[string]bool)
	roots := append([]string(nil), scanPaths...)
	roots = append(roots, ".")
	for _, root := range roots {
		base := root
		if info, err := os.Stat(root); err == nil && !info.IsDir() {
			base = filepath.Dir(root)
		}
		for _, rel := range []string{
			"baseline.xml",
			"krit-baseline.xml",
			filepath.Join("build", "reports", "krit", "baseline.xml"),
		} {
			candidate := filepath.Join(base, rel)
			if !seen[candidate] {
				seen[candidate] = true
				candidates = append(candidates, candidate)
			}
		}
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("baseline-audit requires --baseline or a default baseline file (%s)", strings.Join(candidates, ", "))
}

func collectBaselineAuditFiles(scanPaths []string) map[string]bool {
	knownFiles := make(map[string]bool)
	for _, root := range scanPaths {
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			abs, err := filepath.Abs(root)
			if err != nil {
				abs = root
			}
			knownFiles[abs] = true
			continue
		}
		if walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
			}
			if info.IsDir() {
				switch info.Name() {
				case ".git", "build", "node_modules", ".idea", ".gradle", "out", ".kotlin", "target", "third-party", "third_party", "vendor", "external":
					return filepath.SkipDir
				}
				return nil
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				abs = path
			}
			knownFiles[abs] = true
			return nil
		}); walkErr != nil {
			fmt.Fprintf(os.Stderr, "krit: warning: walk %s: %v\n", root, walkErr)
		}
	}
	return knownFiles
}

func baselineEntryFileExists(entry scanner.BaselineEntry, baselinePath, basePath string, knownFiles map[string]bool) bool {
	if entry.Path == "" {
		return false
	}

	path := filepath.FromSlash(entry.Path)
	candidates := []string{}
	if filepath.IsAbs(path) {
		candidates = append(candidates, path)
	} else {
		if basePath != "" {
			candidates = append(candidates, filepath.Join(basePath, path))
		}
		candidates = append(candidates, filepath.Join(filepath.Dir(baselinePath), path))
	}
	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			abs = candidate
		}
		if knownFiles[abs] {
			return true
		}
		if _, err := os.Stat(abs); err == nil {
			return true
		}
	}

	if strings.Contains(entry.Path, "/") || strings.Contains(entry.Path, `\`) {
		return false
	}
	for known := range knownFiles {
		if filepath.Base(known) == entry.Path {
			return true
		}
	}
	return false
}
