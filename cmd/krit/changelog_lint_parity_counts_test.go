package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// lintParityCountClaim is an Unreleased CHANGELOG entry that says its rules match Lint on N recorded project fixtures.
type lintParityCountClaim struct {
	line    int      // 1-based line of the entry's first line
	claimed int      // the N the entry states
	names   []string // backticked identifiers in the entry, in order, without repeats
}

var (
	lintParityCountRe = regexp.MustCompile(`(\d+)\s+recorded\s+project\s+fixtures`)
	backtickedNameRe  = regexp.MustCompile("`([A-Za-z][A-Za-z0-9]*)`")
)

// lintParityCountClaims returns the claims in changelog's `## [Unreleased]` section. A released section is history and
// is not re-counted. An entry is a top-level `- ` bullet and its indented continuation lines, sub-bullets included.
func lintParityCountClaims(changelog string) []lintParityCountClaim {
	var claims []lintParityCountClaim
	inUnreleased := false
	var entry []string
	entryLine := 0
	flush := func() {
		if len(entry) == 0 {
			return
		}
		text := strings.Join(entry, " ")
		entry = nil
		m := lintParityCountRe.FindStringSubmatch(text)
		if m == nil {
			return
		}
		n, _ := strconv.Atoi(m[1])
		claim := lintParityCountClaim{line: entryLine, claimed: n}
		seen := map[string]bool{}
		for _, b := range backtickedNameRe.FindAllStringSubmatch(text, -1) {
			if !seen[b[1]] {
				seen[b[1]] = true
				claim.names = append(claim.names, b[1])
			}
		}
		claims = append(claims, claim)
	}
	for i, line := range strings.Split(changelog, "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			flush()
			inUnreleased = strings.HasPrefix(line, "## [Unreleased]")
		case !inUnreleased:
		case strings.HasPrefix(line, "- "):
			flush()
			entry, entryLine = []string{strings.TrimSpace(line)}, i+1
		case len(entry) > 0 && strings.TrimSpace(line) != "" && (line[0] == ' ' || line[0] == '\t'):
			entry = append(entry, strings.TrimSpace(line))
		default:
			flush()
		}
	}
	flush()
	return claims
}

// lintParityCountProblems checks each claim against the cases TestAndroidLintParityFixtures replays: the
// <fixtures>/<RuleID>/<case>/lint-32.4.1.txt files of the rules the entry names. A backticked name without a fixture
// directory (a Lint detector, a script) does not count.
func lintParityCountProblems(changelog, fixtures string) ([]string, error) {
	var problems []string
	for _, claim := range lintParityCountClaims(changelog) {
		total := 0
		var parts []string
		for _, name := range claim.names {
			if info, err := os.Stat(filepath.Join(fixtures, name)); err != nil || !info.IsDir() {
				continue
			}
			cases, err := filepath.Glob(filepath.Join(fixtures, name, "*", "lint-32.4.1.txt"))
			if err != nil {
				return nil, err
			}
			total += len(cases)
			parts = append(parts, fmt.Sprintf("%s %d", name, len(cases)))
		}
		switch {
		case len(parts) == 0:
			problems = append(problems, fmt.Sprintf("CHANGELOG.md:%d says %d recorded project fixtures but names no rule "+
				"with a tests/fixtures/android-lint/<RuleID>/ directory. Name the rules in backticks.", claim.line, claim.claimed))
		case total != claim.claimed:
			problems = append(problems, fmt.Sprintf("CHANGELOG.md:%d says %d recorded project fixtures, but the rules it "+
				"names have %d (%s). Change the entry's count to %d, or name the rule whose fixtures it leaves out.",
				claim.line, claim.claimed, total, strings.Join(parts, ", "), total))
		}
	}
	return problems, nil
}

// TestChangelogLintParityFixtureCounts keeps each Unreleased CHANGELOG count of recorded Lint parity fixtures equal to
// the cases on disk. Reviews kept recounting them by hand, and two entries had landed miscounted.
func TestChangelogLintParityFixtureCounts(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	problems, err := lintParityCountProblems(string(data), filepath.Join("..", "..", "tests", "fixtures", "android-lint"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

func TestLintParityCountProblems(t *testing.T) {
	fixtures := t.TempDir()
	for _, c := range []string{"Alpha/positive", "Alpha/negative", "Beta/positive", "Beta/unrecorded"} {
		dir := filepath.Join(fixtures, filepath.FromSlash(c))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if filepath.Base(c) == "unrecorded" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, "lint-32.4.1.txt"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const changelog = `# Changelog

## [Unreleased]

### Added

- ` + "`Alpha` and `Beta`, ports of Lint's `AlphaDetector`. Findings match Lint on 3 recorded project" + `
  fixtures (#1).
- ` + "`Alpha`" + `, again:
  - a sub-bullet says it matches Lint on 1 recorded project fixtures.
- ` + "`Beta`" + ` matches Lint on 2 recorded project fixtures.
- Lint on 1 recorded project fixtures, naming ` + "`scripts`" + ` only.
- ` + "`Alpha`" + ` with no count.

## [0.1.0] - 2026-01-01

- ` + "`Alpha`" + ` matches Lint on 9 recorded project fixtures.
`
	problems, err := lintParityCountProblems(changelog, fixtures)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"CHANGELOG.md:9 says 1 recorded project fixtures, but the rules it names have 2 (Alpha 2). Change the entry's " +
			"count to 2, or name the rule whose fixtures it leaves out.",
		"CHANGELOG.md:11 says 2 recorded project fixtures, but the rules it names have 1 (Beta 1). Change the entry's " +
			"count to 1, or name the rule whose fixtures it leaves out.",
		"CHANGELOG.md:12 says 1 recorded project fixtures but names no rule with a tests/fixtures/android-lint/<RuleID>/ " +
			"directory. Name the rules in backticks.",
	}
	if got := strings.Join(problems, "\n"); got != strings.Join(want, "\n") {
		t.Errorf("problems differ\n--- got\n%s\n--- want\n%s", got, strings.Join(want, "\n"))
	}
}
