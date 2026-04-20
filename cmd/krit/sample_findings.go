package main

import (
	"bufio"
	"fmt"
	"hash/fnv"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/scanner"
)

// runSampleFindingsColumns prints a deterministic random sample of findings
// matching ruleName, each accompanied by surrounding source context. It is
// used by the --sample-rule CLI flag to help with false-positive hunting.
//
// Returns the process exit code:
//   - 0 on success (including the "zero matching findings but asked-for rule exists"
//     case where we still want a clean exit so callers can script against it).
//   - 2 when no findings at all matched the requested rule name (mirrors the
//     unknown-format error path).
func runSampleFindingsColumns(columns *scanner.FindingColumns, ruleName string, count int, contextLines int, basePath string) int {
	if count < 0 {
		count = 0
	}
	if contextLines < 0 {
		contextLines = 0
	}

	// Filter to this rule.
	var matching []int
	totalFindings := 0
	if columns != nil {
		totalFindings = columns.Len()
		for i := 0; i < columns.Len(); i++ {
			if columns.RuleAt(i) == ruleName {
				matching = append(matching, i)
			}
		}
	}

	total := len(matching)
	if total == 0 {
		fmt.Fprintf(os.Stderr, "error: no findings for rule '%s' (total findings: %d)\n", ruleName, totalFindings)
		return 2
	}

	// Deterministic shuffle seeded by ruleName|count.
	seedKey := ruleName + "|" + fmt.Sprintf("%d", count)
	h := fnv.New64a()
	_, _ = h.Write([]byte(seedKey))
	//nolint:gosec // math/rand is intentional for deterministic sampling.
	rng := rand.New(rand.NewSource(int64(h.Sum64())))
	rng.Shuffle(len(matching), func(i, j int) {
		matching[i], matching[j] = matching[j], matching[i]
	})

	n := count
	if n > total {
		n = total
	}
	sample := matching[:n]

	absBase, _ := filepath.Abs(basePath)

	for i, row := range sample {
		file := columns.FileAt(row)
		line := columns.LineAt(row)
		col := columns.ColumnAt(row)
		message := columns.MessageAt(row)

		relPath := file
		if absBase != "" {
			if abs, err := filepath.Abs(file); err == nil {
				if rp, err := filepath.Rel(absBase, abs); err == nil {
					relPath = rp
				}
			}
		}

		fmt.Printf("=== %s:%d:%d — %s ===\n", relPath, line, col, ruleName)
		fmt.Printf("  msg: %s\n", message)

		lines, err := readFileLines(file)
		if err != nil {
			fmt.Printf("  (could not read source: %v)\n", err)
		} else {
			start := line - contextLines
			if start < 1 {
				start = 1
			}
			end := line + contextLines
			if end > len(lines) {
				end = len(lines)
			}
			for ln := start; ln <= end; ln++ {
				marker := "  "
				if ln == line {
					marker = ">>"
				}
				fmt.Printf("%s%5d   %s\n", marker, ln, lines[ln-1])
			}
		}

		if i < len(sample)-1 {
			fmt.Println()
		}
	}

	fmt.Println()
	fmt.Printf("sampled %d of %d total for rule '%s'\n", n, total, ruleName)
	return 0
}

// readFileLines reads a file and returns its lines without trailing newlines.
func readFileLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scan.Scan() {
		lines = append(lines, scan.Text())
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
