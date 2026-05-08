package ruleslinter

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// AnalyzeSubpackageTopology enforces the per-domain rule subpackage
// dependency contract: every Go subpackage under internal/rules/ may
// import a curated set of foundation packages and must not import the
// parent rules package or any sibling subpackage. This keeps domains
// isolated from each other and from the registry plumbing in rules,
// preventing cross-domain helper accretion.
//
// rulesParentDir should point at internal/rules. The function walks one
// level deep; nested subpackages (e.g. internal/rules/foo/bar) are not
// checked.
//
// The allowed set is broad: every direct subpackage of
// internal/analyzers, plus internal/scanner, internal/rules/v2,
// internal/rules/base, internal/typeinfer, internal/oracle,
// internal/javafacts, internal/librarymodel, internal/experiment,
// internal/android, internal/module, internal/cache, internal/config,
// and the standard library. Anything else under
// github.com/kaeawc/krit/ is treated as forbidden — including
// internal/rules itself and any sibling domain.
func AnalyzeSubpackageTopology(rulesParentDir string) ([]Violation, error) {
	entries, err := os.ReadDir(rulesParentDir)
	if err != nil {
		return nil, fmt.Errorf("ruleslinter: read rules dir: %w", err)
	}
	var violations []Violation
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// api, v2, base, and semantics are foundation packages — they can
		// import whatever they need; they are not domain rule packages.
		// (v2 was renamed to api in #1210; both names are tolerated until
		// any remaining v2 directories are removed.)
		switch e.Name() {
		case "api", "v2", "base", "semantics":
			continue
		}
		subDir := filepath.Join(rulesParentDir, e.Name())
		v, err := analyzeSubpackageImports(subDir, e.Name())
		if err != nil {
			return nil, err
		}
		violations = append(violations, v...)
	}
	return violations, nil
}

func analyzeSubpackageImports(dir, packageName string) ([]Violation, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("ruleslinter: read %s: %w", dir, err)
	}
	fset := token.NewFileSet()
	var violations []Violation
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, f.Name())
		parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return nil, fmt.Errorf("ruleslinter: parse %s: %w", path, err)
		}
		for _, imp := range parsed.Imports {
			raw, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if reason, forbidden := classifyImport(raw, packageName); forbidden {
				violations = append(violations, Violation{
					RuleID:   packageName,
					Position: fset.Position(imp.Pos()),
					Message:  reason,
				})
			}
		}
	}
	return violations, nil
}

const krit = "github.com/kaeawc/krit/"

// classifyImport reports whether `path` is forbidden for a domain rule
// subpackage named `currentPackage`, with a reason string.
func classifyImport(path, currentPackage string) (string, bool) {
	if !strings.HasPrefix(path, krit) {
		// External / standard-library imports are always allowed.
		return "", false
	}
	rel := strings.TrimPrefix(path, krit)

	// Foundation tier — always allowed.
	switch rel {
	case "internal/scanner",
		"internal/rules/api",
		"internal/rules/v2",
		"internal/rules/base",
		"internal/rules/semantics",
		"internal/typeinfer",
		"internal/oracle",
		"internal/javafacts",
		"internal/librarymodel",
		"internal/experiment",
		"internal/android",
		"internal/module",
		"internal/cache",
		"internal/config",
		"internal/logger",
		"internal/perf":
		return "", false
	}
	if strings.HasPrefix(rel, "internal/analyzers/") {
		return "", false
	}

	// The parent rules package is forbidden — it imports us, not the
	// other way round.
	if rel == "internal/rules" {
		return "domain rule subpackage must not import the parent rules package (creates cycle once parent side-effect-imports the subpackage)", true
	}

	// Sibling domain subpackages are forbidden — share helpers via
	// internal/analyzers/* instead of cross-importing.
	if strings.HasPrefix(rel, "internal/rules/") {
		other := strings.TrimPrefix(rel, "internal/rules/")
		if i := strings.Index(other, "/"); i >= 0 {
			other = other[:i]
		}
		if other != currentPackage {
			return fmt.Sprintf("domain rule subpackage %q must not import sibling subpackage %q; promote shared helpers to internal/analyzers/* instead", currentPackage, other), true
		}
		return "", false
	}

	// Other internal packages are allowed by default; tighten this list as
	// the topology stabilizes.
	return "", false
}
