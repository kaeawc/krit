package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// newExperimentOpts carries the inputs for the scaffold command.
type newExperimentOpts struct {
	Name        string
	Description string
	Intent      string
	TargetRules []string
	WireFile    string // relative to repo root
}

var experimentNameRE = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// runNewExperimentScaffold creates a new experiment entry and wires an import
// into the requested rule file. Returns the process exit code.
func runNewExperimentScaffold(opts newExperimentOpts) int {
	// 1. Validate inputs.
	if opts.Name == "" {
		fmt.Fprintln(os.Stderr, "error: -new-experiment requires a name")
		return 2
	}
	if !experimentNameRE.MatchString(opts.Name) {
		fmt.Fprintf(os.Stderr, "error: experiment name %q must be kebab-case (lowercase letters/digits/hyphens, starts with a letter)\n", opts.Name)
		return 2
	}
	if strings.TrimSpace(opts.Description) == "" {
		fmt.Fprintln(os.Stderr, "error: -new-experiment-description is required")
		return 2
	}
	if opts.Intent == "" {
		opts.Intent = "fp-reduction"
	}
	switch opts.Intent {
	case "fp-reduction", "performance":
	default:
		fmt.Fprintf(os.Stderr, "error: -new-experiment-intent %q must be one of: fp-reduction, performance\n", opts.Intent)
		return 2
	}
	if len(opts.TargetRules) == 0 {
		fmt.Fprintln(os.Stderr, "error: -new-experiment-target-rules is required (comma-separated)")
		return 2
	}
	if opts.WireFile == "" {
		fmt.Fprintln(os.Stderr, "error: -new-experiment-wire-file is required")
		return 2
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: locating repo root: %v\n", err)
		return 2
	}

	wireAbs := opts.WireFile
	if !filepath.IsAbs(wireAbs) {
		wireAbs = filepath.Join(repoRoot, opts.WireFile)
	}
	wireAbs = filepath.Clean(wireAbs)
	if !strings.HasSuffix(wireAbs, ".go") {
		fmt.Fprintf(os.Stderr, "error: -new-experiment-wire-file %q must end in .go\n", opts.WireFile)
		return 2
	}
	// Must live under repoRoot.
	relFromRoot, err := filepath.Rel(repoRoot, wireAbs)
	if err != nil || strings.HasPrefix(relFromRoot, "..") {
		fmt.Fprintf(os.Stderr, "error: -new-experiment-wire-file %q must be inside the repo root %s\n", opts.WireFile, repoRoot)
		return 2
	}
	if _, err := os.Stat(wireAbs); err != nil {
		fmt.Fprintf(os.Stderr, "error: wire file %s does not exist\n", wireAbs)
		return 2
	}

	catalogPath := filepath.Join(repoRoot, "internal", "experiment", "experiment.go")
	catalogBytes, err := os.ReadFile(catalogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading %s: %v\n", catalogPath, err)
		return 2
	}
	catalog := string(catalogBytes)

	// 2. Duplicate check — text search for Name: "NAME".
	if experimentNameRegistered(catalog, opts.Name) {
		fmt.Fprintf(os.Stderr, "error: experiment '%s' already exists\n", opts.Name)
		return 2
	}

	// 3. Append a new entry to knownDefinitions.
	updatedCatalog, err := appendExperimentDefinition(catalog, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: updating experiment catalog: %v\n", err)
		return 2
	}
	if err := os.WriteFile(catalogPath, []byte(updatedCatalog), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing %s: %v\n", catalogPath, err)
		return 2
	}

	// 4. Ensure wire file imports internal/experiment.
	wireBytes, err := os.ReadFile(wireAbs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading %s: %v\n", wireAbs, err)
		return 2
	}
	wireSource := string(wireBytes)
	importAdded := false
	if !hasExperimentImport(wireSource) {
		updatedWire, err := ensureExperimentImport(wireSource)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: adding experiment import to %s: %v\n", wireAbs, err)
			return 2
		}
		if err := os.WriteFile(wireAbs, []byte(updatedWire), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error: writing %s: %v\n", wireAbs, err)
			return 2
		}
		importAdded = true
	}

	// 5. Print "next steps" guidance.
	wireLabel := relFromRoot
	importStatus := "experiment import already present"
	if importAdded {
		importStatus = "experiment import added"
	}
	fmt.Printf("Scaffolded experiment '%s'.\n", opts.Name)
	fmt.Printf("  catalog:    internal/experiment/experiment.go\n")
	fmt.Printf("  wire file:  %s   (%s)\n", wireLabel, importStatus)
	fmt.Printf("  target:     %s\n", strings.Join(opts.TargetRules, ", "))
	fmt.Printf("  intent:     %s\n", opts.Intent)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Open %s and add:\n", wireLabel)
	fmt.Printf("       if experiment.Enabled(\"%s\") && <your-condition> {\n", opts.Name)
	fmt.Printf("           return nil\n")
	fmt.Printf("       }\n")
	fmt.Printf("     inside the check function for the target rule(s).\n")
	fmt.Printf("  2. Run:\n")
	fmt.Printf("       krit -experiment-matrix=baseline,singles \\\n")
	fmt.Printf("            -experiment-candidates=%s \\\n", opts.Name)
	fmt.Printf("            -experiment-targets=/path/to/target -f=plain\n")
	fmt.Printf("  3. Commit with a \"Add %s experiment\" message.\n", opts.Name)

	// 6. go build ./... to catch malformed edits.
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "error: go build ./... failed after scaffold:\n%s\n", string(out))
		return 3
	}
	return 0
}

// findRepoRoot walks upward from the current working directory looking for go.mod.
func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above %s", wd)
		}
		dir = parent
	}
}

// experimentNameRegistered checks whether the catalog file already contains an
// entry with the given Name. Uses a simple text search so it is tolerant of
// extra struct fields (e.g. Status) added by concurrent work.
func experimentNameRegistered(catalog, name string) bool {
	needle := fmt.Sprintf(`Name: %q`, name)
	return strings.Contains(catalog, needle)
}

// appendExperimentDefinition inserts a new Definition literal immediately
// before the closing brace of the knownDefinitions slice literal. It is
// resilient to unknown / additional fields in the Definition struct.
func appendExperimentDefinition(catalog string, opts newExperimentOpts) (string, error) {
	const marker = "var knownDefinitions = []Definition{"
	start := strings.Index(catalog, marker)
	if start < 0 {
		return "", fmt.Errorf("could not find %q in experiment.go", marker)
	}
	// Find matching closing brace (line starting with "}" at column 0 after start).
	// Simplest robust heuristic: scan forward for "\n}\n" after the marker.
	searchFrom := start + len(marker)
	rel := strings.Index(catalog[searchFrom:], "\n}\n")
	if rel < 0 {
		// Also try with trailing newline variations.
		rel = strings.Index(catalog[searchFrom:], "\n}")
		if rel < 0 {
			return "", fmt.Errorf("could not find closing brace of knownDefinitions slice")
		}
	}
	closeIdx := searchFrom + rel + 1 // position of '}'

	// Determine indentation from the last non-blank line before closeIdx.
	before := catalog[:closeIdx]
	lines := strings.Split(before, "\n")
	indent := "\t"
	for i := len(lines) - 2; i >= 0; i-- {
		trimmed := strings.TrimRight(lines[i], " \t")
		if trimmed == "" {
			continue
		}
		// Extract leading whitespace.
		lead := trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, " \t"))]
		if lead != "" {
			indent = lead
		}
		break
	}

	targets := make([]string, 0, len(opts.TargetRules))
	for _, r := range opts.TargetRules {
		targets = append(targets, fmt.Sprintf("%q", r))
	}
	entry := fmt.Sprintf("%s{Name: %q, Description: %q, Intent: %q, TargetRules: []string{%s}},\n",
		indent, opts.Name, opts.Description, opts.Intent, strings.Join(targets, ", "))

	return catalog[:closeIdx] + entry + catalog[closeIdx:], nil
}

// hasExperimentImport reports whether the source already imports the
// internal/experiment package.
func hasExperimentImport(src string) bool {
	return strings.Contains(src, `"github.com/kaeawc/krit/internal/experiment"`)
}

// ensureExperimentImport adds the internal/experiment import inside an
// existing `import (` block, alphabetically within the block that contains
// other kaeawc/krit imports when possible.
func ensureExperimentImport(src string) (string, error) {
	importStart := strings.Index(src, "\nimport (")
	if importStart < 0 {
		return "", fmt.Errorf("no `import (` block found")
	}
	blockOpen := importStart + len("\nimport (")
	blockEnd := strings.Index(src[blockOpen:], "\n)")
	if blockEnd < 0 {
		return "", fmt.Errorf("unterminated import block")
	}
	blockEndAbs := blockOpen + blockEnd
	block := src[blockOpen:blockEndAbs]

	const imp = `"github.com/kaeawc/krit/internal/experiment"`

	// Split block into lines, find correct insertion spot.
	lines := strings.Split(block, "\n")
	insertAt := -1
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, `"github.com/kaeawc/krit/`) {
			continue
		}
		if trim > `"github.com/kaeawc/krit/internal/experiment"` {
			insertAt = i
			break
		}
	}
	// If we never found a later kaeawc/krit import, place after the last one.
	if insertAt < 0 {
		for i, line := range lines {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, `"github.com/kaeawc/krit/`) {
				insertAt = i + 1
			}
		}
	}
	// Fall back: insert before closing brace (end of block).
	if insertAt < 0 {
		insertAt = len(lines) - 1
		if insertAt < 0 {
			insertAt = 0
		}
	}

	newLine := "\t" + imp
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:insertAt]...)
	newLines = append(newLines, newLine)
	newLines = append(newLines, lines[insertAt:]...)
	newBlock := strings.Join(newLines, "\n")

	return src[:blockOpen] + newBlock + src[blockEndAbs:], nil
}
