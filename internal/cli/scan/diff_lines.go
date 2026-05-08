package scan

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

type changedLineInterval struct {
	start int
	end   int
}

var diffHunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func getChangedLineIntervals(ref string, scanPaths []string) (map[string][]changedLineInterval, error) {
	args := []string{"diff", "--unified=0", "--diff-filter=ACMR", ref, "--"}
	args = append(args, scanPaths...)
	cmd := exec.CommandContext(context.Background(), "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", ref, err)
	}
	return parseChangedLineIntervals(string(out))
}

func parseChangedLineIntervals(diff string) (map[string][]changedLineInterval, error) {
	result := make(map[string][]changedLineInterval)
	var currentFile string
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			if path == "/dev/null" {
				currentFile = ""
				continue
			}
			currentFile = strings.TrimPrefix(path, "b/")
			if abs, err := filepath.Abs(currentFile); err == nil {
				currentFile = abs
			}
			continue
		}
		m := diffHunkRe.FindStringSubmatch(line)
		if m == nil || currentFile == "" {
			continue
		}
		start, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, err
		}
		count := 1
		if m[2] != "" {
			count, err = strconv.Atoi(m[2])
			if err != nil {
				return nil, err
			}
		}
		if count == 0 {
			continue
		}
		result[currentFile] = append(result[currentFile], changedLineInterval{start: start, end: start + count - 1})
	}
	return result, nil
}

func filterColumnsByChangedLines(columns *scanner.FindingColumns, changed map[string][]changedLineInterval) scanner.FindingColumns {
	if columns == nil || columns.Len() == 0 || len(changed) == 0 {
		return scanner.FindingColumns{}
	}
	changedByFileIdx := make([][]changedLineInterval, len(columns.Files))
	for i, file := range columns.Files {
		abs, err := filepath.Abs(file)
		if err != nil {
			abs = file
		}
		changedByFileIdx[i] = changed[abs]
	}
	return columns.FilterRows(func(row int) bool {
		line := columns.LineAt(row)
		for _, interval := range changedByFileIdx[columns.FileIdx[row]] {
			if line >= interval.start && line <= interval.end {
				return true
			}
		}
		return false
	})
}
