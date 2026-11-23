package config

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// AndroidLintImport records the Android Lint issues a project turns off:
// `<issue id="X" severity="ignore"/>` in a lint.xml, and the `disable` lists
// of Gradle `lint { }` / `lintOptions { }` blocks. krit's ports of those
// issues stay inactive unless krit.yml (or an imported detekt config) sets
// the rule's `active` explicitly.
type AndroidLintImport struct {
	// Ignored holds the ignored Android Lint issue IDs.
	Ignored map[string]bool
	// Sources are the files that ignored at least one issue.
	Sources []string
}

// Ignores reports whether the project turns off the Android Lint issue id.
func (a *AndroidLintImport) Ignores(id string) bool {
	return a != nil && a.Ignored[id]
}

// IDs returns the ignored issue IDs, sorted.
func (a *AndroidLintImport) IDs() []string {
	if a == nil {
		return nil
	}
	ids := make([]string, 0, len(a.Ignored))
	for id := range a.Ignored {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// androidLintScanDepth bounds the walk for lint.xml and build scripts.
const androidLintScanDepth = 5

// discoverAndroidLint walks each root for lint.xml files and Gradle build
// scripts and collects the issues they turn off. It returns nil when none
// are found.
func discoverAndroidLint(roots []string) *AndroidLintImport {
	imp := &AndroidLintImport{Ignored: map[string]bool{}}
	seen := map[string]bool{}
	for _, root := range roots {
		dir := root
		if info, err := os.Stat(root); err != nil {
			continue
		} else if !info.IsDir() {
			dir = filepath.Dir(root)
		}
		walkAndroidLintDir(dir, 0, func(path string) {
			if seen[path] {
				return
			}
			seen[path] = true
			var ids []string
			if filepath.Base(path) == "lint.xml" {
				ids = ignoredLintXMLIssues(path)
			} else {
				ids = disabledGradleLintIssues(path)
			}
			if len(ids) == 0 {
				return
			}
			imp.Sources = append(imp.Sources, path)
			for _, id := range ids {
				imp.Ignored[id] = true
			}
		})
	}
	if len(imp.Ignored) == 0 {
		return nil
	}
	sort.Strings(imp.Sources)
	return imp
}

// walkAndroidLintDir visits lint.xml and build.gradle(.kts) files below dir.
// Source sets never hold either, so src/ is skipped along with hidden
// directories and build outputs.
func walkAndroidLintDir(dir string, depth int, visit func(path string)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(dir, name)
		if !e.IsDir() {
			if name == "lint.xml" || name == "build.gradle" || name == "build.gradle.kts" {
				visit(path)
			}
			continue
		}
		if depth >= androidLintScanDepth || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "bazel-") {
			continue
		}
		switch name {
		case "build", "out", "src", "node_modules":
			continue
		}
		walkAndroidLintDir(path, depth+1, visit)
	}
}

// ignoredLintXMLIssues returns the issue IDs a lint.xml sets to
// severity="ignore". Path-scoped `<ignore path=...>` entries do not turn
// an issue off project-wide and are not counted; neither is id="all".
func ignoredLintXMLIssues(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc struct {
		XMLName xml.Name `xml:"lint"`
		Issues  []struct {
			ID       string `xml:"id,attr"`
			Severity string `xml:"severity,attr"`
		} `xml:"issue"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	var ids []string
	for _, issue := range doc.Issues {
		if !strings.EqualFold(strings.TrimSpace(issue.Severity), "ignore") {
			continue
		}
		for _, id := range strings.Split(issue.ID, ",") {
			if id = strings.TrimSpace(id); id != "" && id != "all" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

var (
	gradleLintBlockRe   = regexp.MustCompile(`\b(?:lint|lintOptions)\s*\{`)
	gradleLintDisableRe = regexp.MustCompile(`\bdisable\b[^\n]*`)
	gradleLintIDRe      = regexp.MustCompile(`["']([A-Za-z][A-Za-z0-9_]*)["']`)
)

// disabledGradleLintIssues returns the issue IDs named by `disable` in the
// lint { } / lintOptions { } blocks of a build script:
// `disable += listOf("A", "B")`, `disable += "A"`, `disable.add("A")`,
// `disable("A")`, Groovy `disable 'A', 'B'`.
func disabledGradleLintIssues(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	src := string(data)
	var ids []string
	for _, loc := range gradleLintBlockRe.FindAllStringIndex(src, -1) {
		body := braceBody(src, loc[1]-1)
		for _, line := range gradleLintDisableRe.FindAllString(body, -1) {
			if i := strings.Index(line, "//"); i >= 0 {
				line = line[:i]
			}
			for _, m := range gradleLintIDRe.FindAllStringSubmatch(line, -1) {
				ids = append(ids, m[1])
			}
		}
	}
	return ids
}

// braceBody returns the text between the '{' at open and its matching '}'.
func braceBody(src string, open int) string {
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[open+1 : i]
			}
		}
	}
	return src[open+1:]
}
