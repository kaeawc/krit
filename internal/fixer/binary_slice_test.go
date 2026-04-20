package fixer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/scanner"
)

// Test-only slice-taking binary-fix helpers. Production code uses
// ApplyBinaryFixesBatchColumns (pipeline/fixup.go); the slice variants
// exist so binary_test.go can drive the fix pipeline with literal
// []scanner.Finding fixtures.

// ApplyBinaryFixes processes binary fix findings one at a time in the order
// they appear. Retained for tests that exercise per-finding error semantics.
func ApplyBinaryFixes(findings []scanner.Finding, dryRun bool) (applied int, errors []error) {
	for _, f := range findings {
		if f.BinaryFix == nil {
			continue
		}
		if f.BinaryFix.HintOnly {
			continue
		}
		switch f.BinaryFix.Type {
		case scanner.BinaryFixConvertWebP:
			err := convertToWebP(f.BinaryFix.SourcePath, f.BinaryFix.TargetPath, dryRun)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			applied++
			if f.BinaryFix.DeleteSource && !dryRun {
				if err := os.Remove(f.BinaryFix.SourcePath); err != nil {
					errors = append(errors, fmt.Errorf("delete source %s: %w", f.BinaryFix.SourcePath, err))
				}
			}
		case scanner.BinaryFixDeleteFile:
			if dryRun {
				applied++
				continue
			}
			if err := os.Remove(f.BinaryFix.SourcePath); err != nil {
				errors = append(errors, fmt.Errorf("delete %s: %w", f.BinaryFix.SourcePath, err))
			} else {
				applied++
			}
		case scanner.BinaryFixCreateFile:
			if dryRun {
				applied++
				continue
			}
			dir := filepath.Dir(f.BinaryFix.TargetPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				errors = append(errors, fmt.Errorf("mkdir %s: %w", dir, err))
				continue
			}
			if err := os.WriteFile(f.BinaryFix.TargetPath, f.BinaryFix.Content, 0644); err != nil {
				errors = append(errors, fmt.Errorf("create %s: %w", f.BinaryFix.TargetPath, err))
			} else {
				applied++
			}
		case scanner.BinaryFixMoveFile:
			if dryRun {
				applied++
				continue
			}
			dir := filepath.Dir(f.BinaryFix.TargetPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				errors = append(errors, fmt.Errorf("mkdir %s: %w", dir, err))
				continue
			}
			if err := os.Rename(f.BinaryFix.SourcePath, f.BinaryFix.TargetPath); err != nil {
				errors = append(errors, fmt.Errorf("move %s -> %s: %w", f.BinaryFix.SourcePath, f.BinaryFix.TargetPath, err))
			} else {
				applied++
			}
		case scanner.BinaryFixOptimizePNG:
			err := optimizePNG(f.BinaryFix.SourcePath, dryRun)
			if err != nil {
				errors = append(errors, err)
			} else {
				applied++
			}
		}
	}
	return
}

// ApplyBinaryFixesBatch processes all binary fix findings in a safe order:
// first all conversions and optimizations, then file creates/moves, then all
// deletions. Slice counterpart of ApplyBinaryFixesBatchColumns.
func ApplyBinaryFixesBatch(findings []scanner.Finding, dryRun bool, searchDirs ...[]string) (applied int, errors []error) {
	var fixes []*scanner.BinaryFix
	for i := range findings {
		if findings[i].BinaryFix != nil {
			fixes = append(fixes, findings[i].BinaryFix)
		}
	}
	return applyBinaryFixesBatchRaw(fixes, dryRun, searchDirs...)
}
