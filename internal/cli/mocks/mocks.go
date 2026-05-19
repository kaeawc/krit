package mocks

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/strutil"
)

type Report struct {
	TotalMocks int            `json:"totalMocks"`
	ByLibrary  map[string]int `json:"byLibrary"`
	Mocks      []Mock         `json:"mocks"`
	Unused     []Mock         `json:"unused"`
}

type Mock struct {
	Name       string `json:"name"`
	TargetType string `json:"targetType,omitempty"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Library    string `json:"library"`
	Stubbed    bool   `json:"stubbed"`
	Verified   bool   `json:"verified"`
	Interacted bool   `json:"interacted"`
}

var (
	mockkCallRe   = regexp.MustCompile(`(?:val|var)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?::\s*([A-Za-z_][A-Za-z0-9_.$<>?]*))?\s*=\s*(?:io\.mockk\.)?(mockk|spyk)\s*(?:<\s*([A-Za-z_][A-Za-z0-9_.$<>?]*)\s*>)?`)
	mockitoCallRe = regexp.MustCompile(`(?:val|var)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?::\s*([A-Za-z_][A-Za-z0-9_.$<>?]*))?\s*=\s*(?:org\.mockito\.Mockito\.)?(mock)\b\s*(?:<\s*([A-Za-z_][A-Za-z0-9_.$<>?]*)\s*>)?`)
	javaMockRe    = regexp.MustCompile(`(?:[A-Za-z_][A-Za-z0-9_.$<>?]*\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(?:(?:org\.mockito\.)?Mockito\.)?mock\s*\(\s*([A-Za-z_][A-Za-z0-9_.$]*)\.class\s*\)`)
	annotatedRe   = regexp.MustCompile(`@(MockK|Mock)\b[\s\S]{0,160}?(?:lateinit\s+)?(?:var|val)?\s*([A-Za-z_][A-Za-z0-9_]*)\s*:\s*([A-Za-z_][A-Za-z0-9_.$<>?]*)`)
)

func Run(args []string) int {
	fs := flag.NewFlagSet("mocks", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	format := fs.String("format", "plain", "Output format: plain or json")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	report, err := Scan(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	default:
		emitPlain(report)
	}
	return 0
}

func Scan(root string) (Report, error) {
	var mocks []Mock
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", ".gradle", ".idea", ".kotlin", "build", "out", "target", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !isTestPath(path) || (!strings.HasSuffix(path, ".kt") && !strings.HasSuffix(path, ".java")) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil //nolint:nilerr // skip-and-continue: per-file read/parse error inside Walk callback
		}
		mocks = append(mocks, scanFile(root, path, string(data))...)
		return nil
	})
	if err != nil {
		return Report{}, err
	}
	sort.Slice(mocks, func(i, j int) bool {
		if mocks[i].File != mocks[j].File {
			return mocks[i].File < mocks[j].File
		}
		if mocks[i].Line != mocks[j].Line {
			return mocks[i].Line < mocks[j].Line
		}
		return mocks[i].Name < mocks[j].Name
	})
	report := Report{TotalMocks: len(mocks), ByLibrary: map[string]int{}, Mocks: mocks}
	for _, mock := range mocks {
		report.ByLibrary[mock.Library]++
		if !mock.Stubbed && !mock.Verified && !mock.Interacted {
			report.Unused = append(report.Unused, mock)
		}
	}
	return report, nil
}

func scanFile(root, path, text string) []Mock {
	var out []Mock
	add := func(name, target, library string, offset int) {
		if name == "" {
			return
		}
		mock := Mock{
			Name:       name,
			TargetType: strings.Trim(target, "?"),
			File:       relPath(root, path),
			Line:       lineAt(text, offset),
			Library:    library,
		}
		mock.Stubbed = mockHasCall(text, name, []string{"every", "coEvery", "whenever", "given"})
		mock.Verified = mockHasCall(text, name, []string{"verify", "coVerify", "verifySequence", "verifyOrder"})
		mock.Interacted = mockDirectlyInteracted(text, name)
		out = append(out, mock)
	}
	for _, match := range mockkCallRe.FindAllStringSubmatchIndex(text, -1) {
		groups := submatches(text, match)
		target := firstNonEmpty(groups[4], groups[2])
		add(groups[1], target, "mockk", match[0])
	}
	for _, match := range mockitoCallRe.FindAllStringSubmatchIndex(text, -1) {
		groups := submatches(text, match)
		target := firstNonEmpty(groups[4], groups[2])
		add(groups[1], target, "mockito", match[0])
	}
	for _, match := range javaMockRe.FindAllStringSubmatchIndex(text, -1) {
		groups := submatches(text, match)
		add(groups[1], groups[2], "mockito", match[0])
	}
	for _, match := range annotatedRe.FindAllStringSubmatchIndex(text, -1) {
		groups := submatches(text, match)
		library := "mockito"
		if groups[1] == "MockK" {
			library = "mockk"
		}
		add(groups[2], groups[3], library, match[0])
	}
	return dedupeMocks(out)
}

func submatches(text string, match []int) []string {
	out := make([]string, len(match)/2)
	for i := 0; i < len(match); i += 2 {
		if match[i] >= 0 {
			out[i/2] = text[match[i]:match[i+1]]
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mockHasCall(text, name string, calls []string) bool {
	for _, line := range strings.Split(text, "\n") {
		for _, call := range calls {
			if tokenInText(line, call) && tokenInText(line, name) {
				return true
			}
		}
	}
	return false
}

func mockDirectlyInteracted(text, name string) bool {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*\.`)
	for _, line := range strings.Split(text, "\n") {
		if !re.MatchString(line) {
			continue
		}
		if !strings.Contains(line, "mockk") && !strings.Contains(line, "mock(") && !strings.Contains(line, "@Mock") {
			return true
		}
	}
	return false
}

func tokenInText(text, token string) bool {
	return strutil.ContainsTokenWordBoundary(text, token)
}

func dedupeMocks(in []Mock) []Mock {
	seen := make(map[string]bool, len(in))
	var out []Mock
	for _, mock := range in {
		key := mock.File + "|" + mock.Name + "|" + mock.TargetType + "|" + mock.Library
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, mock)
	}
	return out
}

func emitPlain(report Report) {
	fmt.Printf("Total mocks: %d\n", report.TotalMocks)
	var libs []string
	for lib := range report.ByLibrary {
		libs = append(libs, lib)
	}
	sort.Strings(libs)
	var parts []string
	for _, lib := range libs {
		parts = append(parts, fmt.Sprintf("%s=%d", lib, report.ByLibrary[lib]))
	}
	fmt.Printf("By library: %s\n", strings.Join(parts, " "))
	fmt.Printf("Unused mock targets: %d\n", len(report.Unused))
	for _, mock := range report.Unused {
		fmt.Printf("  %-12s %s:%d (%s) no stubbing, no verification\n", mock.TargetType, mock.File, mock.Line, mock.Library)
	}
}

func lineAt(text string, offset int) int {
	if offset < 0 {
		return 1
	}
	return strings.Count(text[:offset], "\n") + 1
}

func isTestPath(path string) bool {
	slash := filepath.ToSlash(path)
	for _, marker := range []string{"/test/", "/androidTest/", "/commonTest/", "/jvmTest/", "/testFixtures/"} {
		if strings.Contains(slash, marker) {
			return true
		}
	}
	return false
}

func relPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}
