package parity_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

const (
	firGoldenDataRel = "tools/krit-fir/compiler-tests/src/test/data/diagnostic"
	firCheckersRel   = "tools/krit-fir/src/main/kotlin/dev/jasonpearson/krit/fir/checkers"
	// updateGoLinesEnv makes TestFirGoldenGoLines write each golden's
	// go-lines header from the Go rule's output instead of checking it.
	updateGoLinesEnv = "KRIT_UPDATE_GO_LINES"
)

var (
	// goLinesHeaderRe matches the header line; group 1 is its value.
	goLinesHeaderRe = regexp.MustCompile(`^//\s*go-lines:(.*)$`)
	// goLinesEntryRe is one entry: a line number, optionally `xN` for N
	// findings on that line.
	goLinesEntryRe = regexp.MustCompile(`^([1-9][0-9]*)(?:x([2-9]|[1-9][0-9]+))?$`)
	// goldenMarkerRe is AbstractDiagnosticTest's inline marker syntax.
	goldenMarkerRe = regexp.MustCompile(`<!([A-Za-z][A-Za-z0-9_]*)!>(.*?)<!>`)
	firRuleIDRe    = regexp.MustCompile(`override val ruleId = "([A-Za-z0-9_]+)"`)
	packageLineRe  = regexp.MustCompile(`^\s*package\s`)
)

// TestFirGoldenGoLines machine-checks what each FIR golden file says about
// the Go rule it ports. A golden `data/diagnostic/<category>/<RuleId>*.kt`
// marks the lines the FIR checker reports; its `// go-lines:` header lists
// the lines where the Go rule reports on the same source (the file as
// written, header included, with the inline markers stripped, which keeps
// every line number). Markers against go-lines show every FIR/Go divergence
// in the file, so a golden comment's claim about Go ("Go reports this
// because ...", "Go misses ...") is checked, not trusted.
//
// The golden's rule is chosen as AbstractDiagnosticTest chooses it: the
// longest FIR checker ID (from the checker sources, so no jar is needed)
// that prefixes the file name. A golden outside stubs/ whose name starts
// with no checker ID fails: the compiler test would run it with every rule
// enabled and its claims about its own rule would go unchecked. The stubs/
// smoke files and goldens of FIR-only checkers (no Go rule) are exempt and
// must not carry the header. The Go rule runs alone, in-process,
// with source inference, no oracle, and the shipped rule options
// (config/default-krit.yml, applied by TestMain), like TestFirFixtureParity's
// Go side, on a production (non-test) path. With -v, each golden logs its
// divergences as rows for a PR's Divergences table.
//
// Regenerate the headers with
//
//	KRIT_UPDATE_GO_LINES=1 go test ./tests/parity/ -run TestFirGoldenGoLines -count=1
func TestFirGoldenGoLines(t *testing.T) {
	root := repoRoot(t)
	firIDs := firCheckerRuleIDs(t, root)
	goldens, err := filepath.Glob(filepath.Join(root, firGoldenDataRel, "*", "*.kt"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(goldens)
	update := os.Getenv(updateGoLinesEnv) == "1"
	checked := 0
	for _, path := range goldens {
		category := filepath.Base(filepath.Dir(path))
		if category == "stubs" {
			continue // stub smoke files name no rule
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		name := strings.TrimSuffix(filepath.Base(path), ".kt")
		firID := longestPrefixID(name, firIDs)
		var rule *api.Rule
		if firID != "" {
			rule = findGoRule(firID)
		}
		t.Run(category+"/"+name, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(data), "\n")
			headerIdx, header, herr := findGoLinesHeader(lines)
			if firID == "" {
				t.Fatalf("%s names no FIR checker: name a golden `<RuleId>…kt` after the rule it tests, "+
					"so the compiler test runs that rule and this test checks its go-lines", rel)
			}
			if rule == nil {
				if headerIdx >= 0 || herr != nil {
					t.Fatalf("%s carries a go-lines header but names no ported Go rule (FIR checker %q); remove the header", rel, firID)
				}
				return
			}
			if rule.Needs.HasAny(projectScopeNeeds) {
				t.Fatalf("Go rule %q needs project context (Needs=%v); a single golden file cannot run it", rule.ID, rule.Needs)
			}
			// A malformed value is rewritten by the generator; a misplaced or
			// duplicated header needs a hand edit.
			if herr != nil && (!update || headerIdx < 0) {
				t.Fatalf("%s: %v", rel, herr)
			}
			if update {
				updateGoLinesHeader(t, path, rel, category, rule, lines, headerIdx)
				return
			}
			if headerIdx < 0 {
				t.Fatalf("%s has no `// go-lines:` header; every golden of a ported Go rule records the lines where %s reports. "+
					"Generate it: %s=1 go test ./tests/parity/ -run TestFirGoldenGoLines -count=1", rel, rule.ID, updateGoLinesEnv)
			}
			actual := goLineCountsOnGolden(t, category, name, rule, lines)
			if !sameCounts(header, actual) {
				t.Errorf("%s: go-lines header disagrees with the Go rule %s on the marker-stripped golden\n"+
					"  header (line %d): %s\n  Go reports:       %s\n"+
					"Fix any golden comment that claims otherwise about Go, then regenerate the header (%s=1).",
					rel, rule.ID, headerIdx+1, formatGoLines(header), formatGoLines(actual), updateGoLinesEnv)
			}
			logDivergences(t, rel, rule.ID, lines, header)
		})
		if rule != nil {
			checked++
		}
	}
	if checked == 0 {
		t.Fatalf("no golden under %s maps to a ported Go rule; the FIR checker scan or the golden layout changed", firGoldenDataRel)
	}
}

// firCheckerRuleIDs reads every checker's `override val ruleId = "..."`
// from the krit-fir sources, the IDs FirRuleDiscovery finds in the jar.
func firCheckerRuleIDs(t *testing.T, root string) []string {
	t.Helper()
	var ids []string
	err := filepath.WalkDir(filepath.Join(root, firCheckersRel), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".kt") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range firRuleIDRe.FindAllStringSubmatch(string(data), -1) {
			ids = append(ids, m[1])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 {
		t.Fatalf("found no `override val ruleId` under %s", firCheckersRel)
	}
	return ids
}

func longestPrefixID(name string, ids []string) string {
	best := ""
	for _, id := range ids {
		if strings.HasPrefix(name, id) && len(id) > len(best) {
			best = id
		}
	}
	return best
}

// findGoLinesHeader returns the index of the go-lines header (-1 when absent)
// and its parsed value. The header must appear once, above the package
// declaration.
func findGoLinesHeader(lines []string) (int, map[int]int, error) {
	idx, pkg := -1, -1
	var value string
	for i, line := range lines {
		if pkg < 0 && packageLineRe.MatchString(line) {
			pkg = i
		}
		m := goLinesHeaderRe.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		if idx >= 0 {
			return -1, nil, fmt.Errorf("more than one go-lines header (lines %d and %d)", idx+1, i+1)
		}
		idx, value = i, m[1]
	}
	if idx < 0 {
		return -1, nil, nil
	}
	if pkg >= 0 && idx > pkg {
		return -1, nil, fmt.Errorf("go-lines header on line %d is below the package declaration; move it above", idx+1)
	}
	counts, err := parseGoLines(value)
	if err != nil {
		return idx, nil, fmt.Errorf("go-lines header on line %d: %w", idx+1, err)
	}
	return idx, counts, nil
}

func parseGoLines(value string) (map[int]int, error) {
	value = strings.TrimSpace(value)
	counts := map[int]int{}
	if value == "none" {
		return counts, nil
	}
	if value == "" {
		return nil, fmt.Errorf("empty value; write `none` when the Go rule reports nothing")
	}
	prev := 0
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		m := goLinesEntryRe.FindStringSubmatch(entry)
		if m == nil {
			return nil, fmt.Errorf("bad entry %q; want `<line>` or `<line>x<count>`, comma-separated, or `none`", entry)
		}
		line, _ := strconv.Atoi(m[1])
		if line <= prev {
			return nil, fmt.Errorf("entries must be strictly increasing line numbers; %d follows %d", line, prev)
		}
		prev = line
		n := 1
		if m[2] != "" {
			n, _ = strconv.Atoi(m[2])
		}
		counts[line] = n
	}
	return counts, nil
}

func formatGoLines(counts map[int]int) string {
	if len(counts) == 0 {
		return "none"
	}
	lines := make([]int, 0, len(counts))
	for line := range counts {
		lines = append(lines, line)
	}
	sort.Ints(lines)
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		if n := counts[line]; n > 1 {
			parts = append(parts, fmt.Sprintf("%dx%d", line, n))
		} else {
			parts = append(parts, strconv.Itoa(line))
		}
	}
	return strings.Join(parts, ", ")
}

// stripGoldenMarkers removes the inline markers line by line, as
// AbstractDiagnosticTest.parseMarkers does, so line numbers are unchanged.
func stripGoldenMarkers(lines []string) string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = goldenMarkerRe.ReplaceAllString(line, "${2}")
	}
	return strings.Join(out, "\n")
}

// goLineCountsOnGolden runs rule alone on the marker-stripped golden and
// returns its findings per line. The file is parsed in memory under a
// production source path: the golden's own path is under src/test and
// compiler-tests, which scanner.IsTestFile classifies as test code, while
// the golden compile treats nothing as a test file.
func goLineCountsOnGolden(t *testing.T, category, name string, rule *api.Rule, lines []string) map[int]int {
	t.Helper()
	source := []byte(stripGoldenMarkers(lines))
	path := filepath.Join("golden", "src", "main", "kotlin", category, name+".kt")
	parser := scanner.GetKotlinParser()
	tree, err := parser.ParseCtx(context.Background(), nil, source)
	scanner.PutKotlinParser(parser)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	file := scanner.NewParsedFile(path, source, tree)
	if scanner.IsTestFile(file.Path) {
		t.Fatalf("golden scan path %s classifies as a test file", file.Path)
	}
	var resolver typeinfer.TypeResolver
	if rule.Needs.Has(api.NeedsResolver) {
		r := typeinfer.NewResolver()
		r.IndexFilesParallel([]*scanner.File{file}, 1)
		resolver = r
	}
	cols, stats := rules.NewDispatcher([]*api.Rule{rule}, resolver).RunWithStats(file)
	if len(stats.Errors) > 0 {
		// An erroring rule emits nothing, which would read as "Go reports
		// no line" and pass a `none` header vacuously.
		t.Fatalf("Go rule %q errored on golden %s: %v", rule.ID, name, stats.Errors)
	}
	return findingLineCounts(cols.Findings(), rule.ID)
}

// updateGoLinesHeader writes the go-lines header from the Go rule's output.
// A new header goes on line 2 after the RENDER_DIAGNOSTICS_FULL_TEXT
// directive, or on line 1 when the file has none; the Go rule runs on the
// file with the header already in place, so its lines are final.
func updateGoLinesHeader(t *testing.T, path, rel, category string, rule *api.Rule, lines []string, headerIdx int) {
	t.Helper()
	name := strings.TrimSuffix(filepath.Base(path), ".kt")
	if headerIdx < 0 {
		headerIdx = 0
		if len(lines) > 0 && strings.TrimSpace(lines[0]) == "// RENDER_DIAGNOSTICS_FULL_TEXT" {
			headerIdx = 1
		}
		lines = append(lines[:headerIdx], append([]string{"// go-lines: none"}, lines[headerIdx:]...)...)
	}
	// The header is a comment, but run to a fixed point in case a rule reads
	// comment text.
	converged := false
	for i := 0; i < 3 && !converged; i++ {
		want := "// go-lines: " + formatGoLines(goLineCountsOnGolden(t, category, name, rule, lines))
		converged = lines[headerIdx] == want
		lines[headerIdx] = want
	}
	if !converged {
		t.Fatalf("%s: the go-lines header did not converge (the Go rule's findings change with the header text, now %q); "+
			"the rule reads comment text, so move or reword the comment it matches", rel, lines[headerIdx])
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %s", rel, lines[headerIdx])
}

// logDivergences logs, under -v, every line where the golden's markers and
// its go-lines header disagree: a marker the header does not list is a
// finding Go misses, and a listed line without a marker is a Go finding FIR
// drops. Each row is a line of the PR's Divergences table (code shape, Go,
// FIR, golden case). Lines where Go reports more than once are listed too,
// since markers record lines, not counts.
func logDivergences(t *testing.T, rel, ruleID string, lines []string, goLines map[int]int) {
	t.Helper()
	fir := map[int]bool{}
	for i, line := range lines {
		for _, m := range goldenMarkerRe.FindAllStringSubmatch(line, -1) {
			if m[1] == ruleID {
				fir[i+1] = true
			}
		}
	}
	all := map[int]bool{}
	for line := range fir {
		all[line] = true
	}
	for line := range goLines {
		all[line] = true
	}
	var rows []int
	for line := range all {
		if !fir[line] || goLines[line] != 1 {
			rows = append(rows, line)
		}
	}
	sort.Ints(rows)
	for _, line := range rows {
		goCol := "no finding"
		if n := goLines[line]; n == 1 {
			goCol = "reports"
		} else if n > 1 {
			goCol = fmt.Sprintf("reports x%d", n)
		}
		firCol := "no finding"
		if fir[line] {
			firCol = "reports"
		}
		code := strings.TrimSpace(stripGoldenMarkers([]string{lines[line-1]}))
		t.Logf("divergence | `%s` | %s | %s | %s:%d", code, goCol, firCol, filepath.Base(rel), line)
	}
}

func TestParseGoLines(t *testing.T) {
	good := map[string]string{
		"none":          "none",
		" 3 ":           "3",
		"3, 7x2, 12":    "3, 7x2, 12",
		"3,7x2,12":      "3, 7x2, 12",
		"1, 10x12, 100": "1, 10x12, 100",
	}
	for in, want := range good {
		counts, err := parseGoLines(in)
		if err != nil {
			t.Errorf("parseGoLines(%q): %v", in, err)
			continue
		}
		if got := formatGoLines(counts); got != want {
			t.Errorf("parseGoLines(%q) formats as %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{"", "0", "3x1", "3x0", "7, 3", "3, 3", "3,", "three", "none, 3", "-1"} {
		if _, err := parseGoLines(in); err == nil {
			t.Errorf("parseGoLines(%q) accepted a malformed value", in)
		}
	}
}

func TestFindGoLinesHeaderPlacement(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		idx   int
		fails bool
	}{
		{"absent", []string{"// RENDER_DIAGNOSTICS_FULL_TEXT", "package test"}, -1, false},
		{"line 2", []string{"// RENDER_DIAGNOSTICS_FULL_TEXT", "// go-lines: 4", "package test", "fun f() = 1"}, 1, false},
		{"below package", []string{"package test", "// go-lines: none"}, -1, true},
		{"duplicated", []string{"// go-lines: none", "// go-lines: none", "package test"}, -1, true},
	}
	for _, c := range cases {
		idx, _, err := findGoLinesHeader(c.lines)
		if (err != nil) != c.fails || (!c.fails && idx != c.idx) {
			t.Errorf("%s: findGoLinesHeader = (%d, %v), want index %d, error %v", c.name, idx, err, c.idx, c.fails)
		}
	}
}
