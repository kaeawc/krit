package fixer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// ValidateBinaryFix checks whether a BinaryFix can be safely applied. For WebP
// conversions it performs three checks:
//
//  1. Animated asset detection: animated GIFs and APNGs cannot be safely
//     converted to static WebP. If the source is animated, an error is returned.
//
//  2. MinSdk compatibility: WebP lossy requires API 14, WebP lossless/transparency
//     requires API 18. If BinaryFix.MinSdk is set and below the threshold, an
//     error is returned.
//
//  3. Direct file reference scan: scans Kotlin, Java, and XML files for literal
//     occurrences of the source file name (e.g. "icon.png"). Android resource
//     references like "@drawable/icon" are safe because they resolve by name
//     without extension, but direct file-name strings would break after conversion.
//
// Returns (nil, error) for safety failures (animated, minSdk).
// Returns (refs, nil) when direct file references are found (caller decides).
// Returns (nil, nil) when the fix is safe.
func ValidateBinaryFix(fix *scanner.BinaryFix, searchDirs []string) ([]android.FileReference, error) {
	if fix == nil || fix.Type != scanner.BinaryFixConvertWebP {
		return nil, nil
	}

	// Animated asset check.
	ext := strings.ToLower(filepath.Ext(fix.SourcePath))
	if ext == ".gif" {
		if animated, _ := android.IsAnimatedGIF(fix.SourcePath); animated {
			return nil, fmt.Errorf("animated GIF cannot be safely converted to static WebP")
		}
	}
	if ext == ".png" {
		if animated, _ := android.IsAnimatedPNG(fix.SourcePath); animated {
			return nil, fmt.Errorf("animated PNG (APNG) cannot be safely converted to static WebP")
		}
	}

	// MinSdk compatibility check.
	// WebP lossy: API 14, WebP lossless/transparency: API 18.
	// We use API 14 as the minimum since cwebp defaults to lossy encoding.
	if fix.MinSdk > 0 && fix.MinSdk < 14 {
		return nil, fmt.Errorf("WebP requires minSdk >= 14, project targets %d", fix.MinSdk)
	}

	// Direct file reference scan.
	baseName := filepath.Base(fix.SourcePath)
	refs := android.ScanFileReferences(searchDirs, baseName)
	return refs, nil
}

// ApplyBinaryFixes processes binary fix findings.
// For WebP conversion, it shells out to cwebp if available.
// Returns the number of fixes applied and any errors encountered.
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
			// If conversion succeeded and DeleteSource is set, remove the original file.
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
// first all conversions and optimizations, then file creates/moves, then all deletions.
// This ensures source files still exist when the conversion step reads them.
// Returns the number of fixes applied and any errors encountered.
//
// searchDirs specifies directories to scan for direct file references before
// applying WebP conversions. If a conversion target has direct file-name
// references (e.g. "icon.png" in source code), the fix is downgraded to
// HintOnly and skipped. Pass nil to skip reference validation.
func ApplyBinaryFixesBatch(findings []scanner.Finding, dryRun bool, searchDirs ...[]string) (applied int, errors []error) {
	var refDirs []string
	if len(searchDirs) > 0 {
		refDirs = searchDirs[0]
	}
	// Partition findings into categories.
	var conversions, creates, moves, deletions, optimizations []scanner.Finding
	for _, f := range findings {
		if f.BinaryFix == nil {
			continue
		}
		if f.BinaryFix.HintOnly {
			continue
		}
		switch f.BinaryFix.Type {
		case scanner.BinaryFixConvertWebP:
			conversions = append(conversions, f)
		case scanner.BinaryFixDeleteFile:
			deletions = append(deletions, f)
		case scanner.BinaryFixCreateFile:
			creates = append(creates, f)
		case scanner.BinaryFixMoveFile:
			moves = append(moves, f)
		case scanner.BinaryFixOptimizePNG:
			optimizations = append(optimizations, f)
		}
	}

	// Track which source paths were successfully converted, so we know
	// it is safe to honour DeleteSource and explicit delete findings.
	converted := make(map[string]bool)

	// First pass: apply all conversions (with safety validation).
	for i := range conversions {
		f := &conversions[i]

		// Always run safety checks (animated assets, minSdk).
		refs, safetyErr := ValidateBinaryFix(f.BinaryFix, refDirs)
		if safetyErr != nil {
			// Safety failure: animated asset or minSdk incompatibility.
			f.BinaryFix.HintOnly = true
			f.BinaryFix.Description = fmt.Sprintf("skipped: %s", safetyErr.Error())
			errors = append(errors, fmt.Errorf(
				"skipped conversion of %s: %s",
				f.BinaryFix.SourcePath, safetyErr.Error(),
			))
			continue
		}
		if len(refs) > 0 {
			// Downgrade to hint-only: direct file references exist.
			f.BinaryFix.HintOnly = true
			f.BinaryFix.Description = fmt.Sprintf(
				"skipped: %d direct reference(s) to %q would break after conversion",
				len(refs), filepath.Base(f.BinaryFix.SourcePath),
			)
			errors = append(errors, fmt.Errorf(
				"skipped conversion of %s: %d direct file reference(s) found",
				f.BinaryFix.SourcePath, len(refs),
			))
			continue
		}
		err := convertToWebP(f.BinaryFix.SourcePath, f.BinaryFix.TargetPath, dryRun)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		applied++
		converted[f.BinaryFix.SourcePath] = true
	}

	// Second pass: optimizations.
	for _, f := range optimizations {
		err := optimizePNG(f.BinaryFix.SourcePath, dryRun)
		if err != nil {
			errors = append(errors, err)
		} else {
			applied++
		}
	}

	// Third pass: file creations.
	for _, f := range creates {
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
	}

	// Fourth pass: file moves.
	for _, f := range moves {
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
	}

	// Fifth pass: delete source files where conversion succeeded and
	// DeleteSource was requested.
	for _, f := range conversions {
		if !f.BinaryFix.DeleteSource {
			continue
		}
		if !converted[f.BinaryFix.SourcePath] {
			continue // conversion failed; do not delete source
		}
		if dryRun {
			continue // already counted in first pass
		}
		if err := os.Remove(f.BinaryFix.SourcePath); err != nil {
			errors = append(errors, fmt.Errorf("delete source %s: %w", f.BinaryFix.SourcePath, err))
		}
	}

	// Sixth pass: explicit BinaryFixDeleteFile findings.
	for _, f := range deletions {
		if dryRun {
			applied++
			continue
		}
		if err := os.Remove(f.BinaryFix.SourcePath); err != nil {
			errors = append(errors, fmt.Errorf("delete %s: %w", f.BinaryFix.SourcePath, err))
		} else {
			applied++
		}
	}

	return
}

// ApplyBinaryFixesBatchColumns applies binary fixes from columnar findings in the
// same safe order as ApplyBinaryFixesBatch while reconstructing only binary-fix rows.
func ApplyBinaryFixesBatchColumns(columns *scanner.FindingColumns, dryRun bool, searchDirs ...[]string) (applied int, errors []error) {
	if columns == nil || columns.Len() == 0 {
		return 0, nil
	}

	findings := make([]scanner.Finding, 0)
	columns.VisitRowsWithBinaryFixes(func(row int) {
		findings = append(findings, scanner.Finding{
			File:      columns.FileAt(row),
			Rule:      columns.RuleAt(row),
			BinaryFix: columns.BinaryFixAt(row),
		})
	})
	return ApplyBinaryFixesBatch(findings, dryRun, searchDirs...)
}

// convertToWebP converts an image file to WebP format using cwebp.
// If dst is empty, generates the target path by replacing the file extension with .webp.
// In dry-run mode, validates that cwebp is available but does not perform conversion.
func convertToWebP(src, dst string, dryRun bool) error {
	if dst == "" {
		dst = strings.TrimSuffix(src, filepath.Ext(src)) + ".webp"
	}
	cwebp, err := exec.LookPath("cwebp")
	if err != nil {
		return fmt.Errorf("cwebp not found: install libwebp for automatic WebP conversion")
	}
	if dryRun {
		return nil
	}
	cmd := exec.Command(cwebp, "-q", "80", src, "-o", dst)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cwebp failed: %w: %s", err, string(output))
	}
	return nil
}

// optimizePNG runs optipng (or pngcrush as fallback) for lossless PNG optimization.
// In dry-run mode, validates that a tool is available but does not perform optimization.
func optimizePNG(src string, dryRun bool) error {
	// Try optipng first, then pngcrush as fallback.
	optipng, err := exec.LookPath("optipng")
	if err == nil {
		if dryRun {
			return nil
		}
		cmd := exec.Command(optipng, "-o2", "-quiet", src)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("optipng failed on %s: %w: %s", src, err, string(output))
		}
		return nil
	}

	pngcrush, err := exec.LookPath("pngcrush")
	if err == nil {
		if dryRun {
			return nil
		}
		tmp := src + ".crush"
		cmd := exec.Command(pngcrush, "-q", src, tmp)
		if output, err := cmd.CombinedOutput(); err != nil {
			os.Remove(tmp) // clean up on failure
			return fmt.Errorf("pngcrush failed on %s: %w: %s", src, err, string(output))
		}
		// Replace original with optimized version.
		if err := os.Rename(tmp, src); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("replace %s with optimized version: %w", src, err)
		}
		return nil
	}

	return fmt.Errorf("no PNG optimizer found: install optipng or pngcrush for lossless PNG optimization")
}
