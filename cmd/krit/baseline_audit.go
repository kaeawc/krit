package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

type baselineAuditIssue struct {
	Entry  scanner.BaselineEntry `json:"entry"`
	Reason string                `json:"reason"`
}

type baselineAuditReport struct {
	BaselinePath string               `json:"baselinePath"`
	ScanPaths    []string             `json:"scanPaths"`
	StaleEntries []baselineAuditIssue `json:"staleEntries"`
}

func runBaselineAudit(findings []scanner.Finding, baseline *scanner.Baseline, baselinePath, basePath string, scanPaths []string, format string) int {
	columns := scanner.CollectFindings(findings)
	return runBaselineAuditColumns(&columns, baseline, baselinePath, basePath, scanPaths, format)
}

func runBaselineAuditColumns(columns *scanner.FindingColumns, baseline *scanner.Baseline, baselinePath, basePath string, scanPaths []string, format string) int {
	liveCapacity := 0
	if columns != nil {
		liveCapacity = columns.Len() * 2
	}
	liveIDs := make(map[string]bool, liveCapacity)
	if columns != nil {
		for i := 0; i < columns.Len(); i++ {
			liveIDs[scanner.BaselineIDAt(columns, i, "", basePath)] = true
			liveIDs[scanner.BaselineIDCompatAt(columns, i, "")] = true
		}
	}

	knownRules := make(map[string]bool, len(rules.Registry))
	for _, rule := range rules.Registry {
		knownRules[rule.Name()] = true
	}

	knownFiles := collectBaselineAuditFiles(scanPaths)
	stale := make([]baselineAuditIssue, 0)
	for _, entry := range baseline.Entries() {
		if liveIDs[entry.ID] {
			continue
		}

		reason := "finding no longer exists"
		switch {
		case entry.Rule == "" || !knownRules[entry.Rule]:
			reason = "rule deleted"
		case !baselineEntryFileExists(entry, baselinePath, basePath, knownFiles):
			reason = "file no longer exists"
		}
		stale = append(stale, baselineAuditIssue{
			Entry:  entry,
			Reason: reason,
		})
	}

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
		}); err != nil {
			fmt.Fprintf(os.Stderr, "error: encode baseline-audit JSON: %v\n", err)
			return 2
		}
		return 0
	}

	fmt.Printf("Baseline audit - %s\n", baselinePath)
	if len(stale) == 0 {
		fmt.Println("No stale baseline entries found.")
		return 0
	}

	fmt.Printf("Dead baseline entries: %d\n", len(stale))
	for _, issue := range stale {
		path := issue.Entry.Path
		if path == "" {
			path = "(unknown path)"
		}
		fmt.Printf("  %s :: %s (%s)\n", path, issue.Entry.Rule, issue.Reason)
	}
	return 0
}

func resolveBaselineAuditPath(explicit string, scanPaths []string) (string, error) {
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
				return nil
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
