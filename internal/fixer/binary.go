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
	var fixes []*scanner.BinaryFix
	for i := range findings {
		if findings[i].BinaryFix != nil {
			fixes = append(fixes, findings[i].BinaryFix)
		}
	}
	return applyBinaryFixesBatchRaw(fixes, dryRun, searchDirs...)
}

// ApplyBinaryFixesBatchColumns applies binary fixes from columnar findings in the
// same safe order as ApplyBinaryFixesBatch, working directly with columnar data.
func ApplyBinaryFixesBatchColumns(columns *scanner.FindingColumns, dryRun bool, searchDirs ...[]string) (applied int, errors []error) {
	if columns == nil || columns.Len() == 0 {
		return 0, nil
	}

	var fixes []*scanner.BinaryFix
	columns.VisitRowsWithBinaryFixes(func(row int) {
		if bf := columns.BinaryFixAt(row); bf != nil {
			fixes = append(fixes, bf)
		}
	})
	return applyBinaryFixesBatchRaw(fixes, dryRun, searchDirs...)
}

// applyBinaryFixesBatchRaw is the core implementation shared by ApplyBinaryFixesBatch
// and ApplyBinaryFixesBatchColumns, operating on raw *BinaryFix pointers.
func applyBinaryFixesBatchRaw(fixes []*scanner.BinaryFix, dryRun bool, searchDirs ...[]string) (applied int, errors []error) {
	var refDirs []string
	if len(searchDirs) > 0 {
		refDirs = searchDirs[0]
	}
	var conversions, creates, moves, deletions, optimizations []*scanner.BinaryFix
	for _, bf := range fixes {
		if bf == nil || bf.HintOnly {
			continue
		}
		switch bf.Type {
		case scanner.BinaryFixConvertWebP:
			conversions = append(conversions, bf)
		case scanner.BinaryFixDeleteFile:
			deletions = append(deletions, bf)
		case scanner.BinaryFixCreateFile:
			creates = append(creates, bf)
		case scanner.BinaryFixMoveFile:
			moves = append(moves, bf)
		case scanner.BinaryFixOptimizePNG:
			optimizations = append(optimizations, bf)
		}
	}
	return applyBinaryFixPartitions(conversions, creates, moves, deletions, optimizations, dryRun, refDirs)
}

// applyBinaryFixPartitions runs the 6-pass ordered apply logic on pre-partitioned fix slices.
// Order: conversions → optimizations → creates → moves → (delete-source for conversions) → explicit deletions.
func applyBinaryFixPartitions(
	conversions, creates, moves, deletions, optimizations []*scanner.BinaryFix,
	dryRun bool,
	refDirs []string,
) (applied int, errors []error) {
	// Track conversions that actually succeeded so Pass 5 only deletes
	// sources whose target was written.
	converted := make(map[string]bool)

	// Pass 1: WebP conversions (with safety validation).
	for _, bf := range conversions {
		refs, safetyErr := ValidateBinaryFix(bf, refDirs)
		if safetyErr != nil {
			bf.HintOnly = true
			bf.Description = fmt.Sprintf("skipped: %s", safetyErr.Error())
			errors = append(errors, fmt.Errorf(
				"skipped conversion of %s: %s",
				bf.SourcePath, safetyErr.Error(),
			))
			continue
		}
		if len(refs) > 0 {
			bf.HintOnly = true
			bf.Description = fmt.Sprintf(
				"skipped: %d direct reference(s) to %q would break after conversion",
				len(refs), filepath.Base(bf.SourcePath),
			)
			errors = append(errors, fmt.Errorf(
				"skipped conversion of %s: %d direct file reference(s) found",
				bf.SourcePath, len(refs),
			))
			continue
		}
		if err := convertToWebP(bf.SourcePath, bf.TargetPath, dryRun); err != nil {
			errors = append(errors, err)
			continue
		}
		applied++
		converted[bf.SourcePath] = true
	}

	// Pass 2: PNG optimizations.
	for _, bf := range optimizations {
		if err := optimizePNG(bf.SourcePath, dryRun); err != nil {
			errors = append(errors, err)
		} else {
			applied++
		}
	}

	// Pass 3: file creates.
	for _, bf := range creates {
		if dryRun {
			applied++
			continue
		}
		dir := filepath.Dir(bf.TargetPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			errors = append(errors, fmt.Errorf("mkdir %s: %w", dir, err))
			continue
		}
		if err := os.WriteFile(bf.TargetPath, bf.Content, 0644); err != nil {
			errors = append(errors, fmt.Errorf("create %s: %w", bf.TargetPath, err))
		} else {
			applied++
		}
	}

	// Pass 4: file moves.
	for _, bf := range moves {
		if dryRun {
			applied++
			continue
		}
		dir := filepath.Dir(bf.TargetPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			errors = append(errors, fmt.Errorf("mkdir %s: %w", dir, err))
			continue
		}
		if err := os.Rename(bf.SourcePath, bf.TargetPath); err != nil {
			errors = append(errors, fmt.Errorf("move %s -> %s: %w", bf.SourcePath, bf.TargetPath, err))
		} else {
			applied++
		}
	}

	// Pass 5: delete source files for conversions that succeeded (DeleteSource flag).
	if !dryRun {
		for _, bf := range conversions {
			if !bf.DeleteSource || !converted[bf.SourcePath] {
				continue
			}
			if err := os.Remove(bf.SourcePath); err != nil {
				errors = append(errors, fmt.Errorf("delete source %s: %w", bf.SourcePath, err))
			}
		}
	}

	// Pass 6: explicit deletions.
	for _, bf := range deletions {
		if dryRun {
			applied++
			continue
		}
		if err := os.Remove(bf.SourcePath); err != nil {
			errors = append(errors, fmt.Errorf("delete %s: %w", bf.SourcePath, err))
		} else {
			applied++
		}
	}
	return
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
