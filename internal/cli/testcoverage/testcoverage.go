package testcoverage

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

type Finding struct {
	Test   string `json:"test"`
	Target string `json:"target"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Kind   string `json:"kind"`
}

func Run(args []string) int {
	fs := flag.NewFlagSet("test-coverage", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	static := fs.Bool("static", false, "Run static test-to-code mapping checks")
	jsonFlag := fs.Bool("json", false, "Emit JSON instead of plain text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !*static {
		fmt.Fprintln(os.Stderr, "usage: krit test-coverage --static [--json]")
		return 1
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	files, err := scanKotlinFiles(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	findings := StaticFindings(root, files)
	if *jsonFlag {
		if err := json.NewEncoder(os.Stdout).Encode(findings); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	} else {
		for _, finding := range findings {
			switch finding.Kind {
			case "class":
				fmt.Printf("%s: claims to cover %s, but the test class never references it\n", finding.Test, finding.Target)
			case "member":
				fmt.Printf("%s: claims to cover %s, but never calls it\n", finding.Test, finding.Target)
			}
		}
	}
	if len(findings) > 0 {
		return 1
	}
	return 0
}

type productionIndex struct {
	classes        map[string]scanner.Symbol
	membersByOwner map[string][]scanner.Symbol
}

func buildProductionIndex(idx *scanner.CodeIndex) productionIndex {
	pi := productionIndex{
		classes:        make(map[string]scanner.Symbol),
		membersByOwner: make(map[string][]scanner.Symbol),
	}
	for _, sym := range idx.Symbols {
		if isTestPath(sym.File) || sym.Language != scanner.LangKotlin {
			continue
		}
		if sym.Kind == "class" && sym.Name != "" && sym.Visibility != "private" {
			pi.classes[sym.Name] = sym
			if sym.FQN != "" {
				pi.classes[sym.FQN] = sym
			}
		}
		if sym.Kind == "function" && sym.Owner != "" && sym.Visibility != "private" {
			pi.membersByOwner[sym.Owner] = append(pi.membersByOwner[sym.Owner], sym)
			if i := strings.LastIndex(sym.Owner, "."); i >= 0 {
				pi.membersByOwner[sym.Owner[i+1:]] = append(pi.membersByOwner[sym.Owner[i+1:]], sym)
			}
		}
	}
	return pi
}

func checkTestClassCoverage(root string, sym scanner.Symbol, file *scanner.File, pi productionIndex, idx *scanner.CodeIndex) []Finding {
	targetName := impliedProductionName(sym.Name)
	if targetName == "" {
		return nil
	}
	target, ok := pi.classes[targetName]
	if !ok {
		return nil
	}
	testText := string(file.Content)
	members := pi.membersByOwner[target.Name]
	if !testTextReferencesTarget(testText, target.Name, members) {
		return []Finding{{
			Test:   sym.Name,
			Target: target.Name,
			File:   relPath(root, sym.File),
			Line:   sym.Line,
			Kind:   "class",
		}}
	}
	return checkMemberCoverage(root, sym, file, target, members, idx)
}

func checkMemberCoverage(root string, sym scanner.Symbol, file *scanner.File, target scanner.Symbol, members []scanner.Symbol, idx *scanner.CodeIndex) []Finding {
	var findings []Finding
	for _, testFn := range idx.Symbols {
		if testFn.File != sym.File || testFn.Kind != "function" || !ownerMatchesSimpleName(testFn.Owner, sym.Name) {
			continue
		}
		member := matchedProductionMember(testFn.Name, members)
		if member.Name == "" {
			continue
		}
		body := symbolText(file, testFn)
		if tokenInText(body, member.Name) {
			continue
		}
		findings = append(findings, Finding{
			Test:   sym.Name + "." + testFn.Name,
			Target: target.Name + "." + member.Name,
			File:   relPath(root, testFn.File),
			Line:   testFn.Line,
			Kind:   "member",
		})
	}
	return findings
}

func StaticFindings(root string, files []*scanner.File) []Finding {
	idx := scanner.BuildIndex(files, runtime.NumCPU())
	filesByPath := make(map[string]*scanner.File, len(files))
	for _, file := range files {
		if file != nil {
			filesByPath[file.Path] = file
		}
	}
	pi := buildProductionIndex(idx)

	var findings []Finding
	for _, sym := range idx.Symbols {
		if sym.Kind != "class" || !isTestPath(sym.File) {
			continue
		}
		file := filesByPath[sym.File]
		if file == nil {
			continue
		}
		findings = append(findings, checkTestClassCoverage(root, sym, file, pi, idx)...)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Test < findings[j].Test
	})
	return findings
}

func ownerMatchesSimpleName(owner, name string) bool {
	return owner == name || strings.HasSuffix(owner, "."+name)
}

func scanKotlinFiles(root string) ([]*scanner.File, error) {
	var out []*scanner.File
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
		if !strings.HasSuffix(path, ".kt") {
			return nil
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err == nil {
			out = append(out, file)
		}
		return nil
	})
	return out, err
}

func impliedProductionName(testClass string) string {
	for _, suffix := range []string{"Tests", "Test", "Spec", "IT"} {
		if strings.HasSuffix(testClass, suffix) && len(testClass) > len(suffix) {
			return strings.TrimSuffix(testClass, suffix)
		}
	}
	return ""
}

func testTextReferencesTarget(text, targetName string, members []scanner.Symbol) bool {
	if tokenInText(text, targetName) {
		return true
	}
	for _, member := range members {
		if tokenInText(text, member.Name) {
			return true
		}
	}
	return false
}

func matchedProductionMember(testName string, members []scanner.Symbol) scanner.Symbol {
	stem := strings.ToLower(strings.TrimSuffix(testName, "s"))
	for _, member := range members {
		name := strings.ToLower(member.Name)
		if stem == name || strings.HasPrefix(stem, name) || strings.HasPrefix(name, stem) {
			return member
		}
	}
	return scanner.Symbol{}
}

func symbolText(file *scanner.File, sym scanner.Symbol) string {
	if file == nil || sym.StartByte < 0 || sym.EndByte <= sym.StartByte || sym.EndByte > len(file.Content) {
		return ""
	}
	return string(file.Content[sym.StartByte:sym.EndByte])
}

func tokenInText(text, token string) bool {
	if token == "" {
		return false
	}
	re := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(token) + `([^A-Za-z0-9_]|$)`)
	return re.MatchString(text)
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
