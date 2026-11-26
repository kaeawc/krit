package firchecks

// coverage.go — the FIR pass's coverage (authoritative vs gated files) for
// the default output and the JSON/SARIF summary (#1573).

import (
	"bufio"
	"bytes"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/scanner"
)

// maxCoverageCauses bounds the gating causes a coverage report lists.
const maxCoverageCauses = 5

var (
	// unresolvedRe matches K2's unresolved-reference message and captures the name.
	unresolvedRe = regexp.MustCompile(`^Unresolved reference:? '([^']+)'`)
	// quotedRe matches the quoted names and types that make otherwise equal
	// messages differ from file to file.
	quotedRe = regexp.MustCompile(`'[^']*'`)
)

// Coverage summarizes stats as an output.FirCoverage. files maps a requested
// path to its parsed file, when one was parsed; other gated files are read
// from disk to find the import an unresolved reference comes from.
func Coverage(stats VerdictStats, files map[string]*scanner.File) output.FirCoverage {
	cov := output.FirCoverage{
		Authoritative: stats.AuthoritativeFiles,
		Gated:         len(stats.GatedFiles),
		Excluded:      stats.ExcludedFiles,
		Withheld:      stats.Withheld,
	}
	counts := map[string]int{}
	for path, reason := range stats.GatedFiles {
		var file *scanner.File
		if files != nil {
			file = files[path]
		}
		counts[gatingCause(firstLine(reason), path, file)]++
	}
	for cause, count := range counts {
		cov.TopCauses = append(cov.TopCauses, output.FirGatingCause{Cause: cause, Count: count})
	}
	sort.Slice(cov.TopCauses, func(i, j int) bool {
		a, b := cov.TopCauses[i], cov.TopCauses[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Cause < b.Cause
	})
	if len(cov.TopCauses) > maxCoverageCauses {
		cov.TopCauses = cov.TopCauses[:maxCoverageCauses]
	}
	return cov
}

// gatingCause groups a file's gating reason with the same reason in other
// files. An unresolved reference becomes the package prefix of the import
// that names it (`Unresolved reference 'androidx.compose'`), so every file
// that misses one library counts as one cause; any other message has its
// quoted names and types blanked.
func gatingCause(reason, path string, file *scanner.File) string {
	if reason == "" {
		return "unknown"
	}
	if strings.HasPrefix(reason, "crashed:") {
		return "compiler crash"
	}
	if m := unresolvedRe.FindStringSubmatch(reason); m != nil {
		if prefix := importPrefix(m[1], importsOf(path, file)); prefix != "" {
			return "Unresolved reference '" + prefix + "'"
		}
		return "Unresolved reference '" + m[1] + "'"
	}
	return quotedRe.ReplaceAllString(reason, "'…'")
}

// importPrefix returns the first import's path up to and including the
// segment name, or "" when no import has that segment.
func importPrefix(name string, imports []string) string {
	for _, imp := range imports {
		segments := strings.Split(imp, ".")
		for i, segment := range segments {
			if segment == name {
				return strings.Join(segments[:i+1], ".")
			}
		}
	}
	return ""
}

// importsOf lists the import paths in the file's header, from the parsed
// file when it has content and otherwise from disk.
func importsOf(path string, file *scanner.File) []string {
	var content []byte
	if file != nil && len(file.Content) > 0 {
		content = file.Content
	} else if data, err := os.ReadFile(path); err == nil {
		content = data
	}
	var imports []string
	sc := bufio.NewScanner(bytes.NewReader(content))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	inBlockComment := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if inBlockComment {
			if strings.Contains(line, "*/") {
				inBlockComment = false
			}
			continue
		}
		switch {
		case line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "@file:") || strings.HasPrefix(line, "package "):
			continue
		case strings.HasPrefix(line, "/*"):
			inBlockComment = !strings.Contains(line, "*/")
			continue
		case strings.HasPrefix(line, "import "):
			imp := strings.TrimSpace(strings.TrimPrefix(line, "import "))
			if i := strings.IndexAny(imp, " \t;"); i >= 0 {
				imp = imp[:i]
			}
			imports = append(imports, strings.Trim(imp, "`"))
			continue
		}
		// The header ends at the first declaration.
		break
	}
	return imports
}
