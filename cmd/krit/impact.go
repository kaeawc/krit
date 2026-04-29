package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

type impactJSON struct {
	Changed []string          `json:"changed"`
	Impact  []impactJSONEntry `json:"impact"`
}

type impactJSONEntry struct {
	File    string   `json:"file"`
	Module  string   `json:"module,omitempty"`
	Symbols []string `json:"symbols"`
}

// runImpactSubcommand implements `krit impact`.
//
// Direct mode:   krit impact <fqn> [<fqn>...]
// File diff:     krit impact --from-file <path>
// Git diff:      krit impact --since <git-ref>
//
// In all modes the input is a set of changed declaration FQNs. The
// command builds the project-wide cross-file index, looks up each
// FQN's simple name in the inverted reference set, and prints the
// union of hit files. Hit files are then expanded transitively: any
// non-private declaration in a hit file is added to the worklist so
// callers of *that* declaration are reported as well.
func runImpactSubcommand(args []string) int {
	fs := flag.NewFlagSet("impact", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonFlag := fs.Bool("json", false, "Emit JSON instead of plain text")
	fromFile := fs.String("from-file", "", "Compute changed-symbol set from this Kotlin file's declarations")
	since := fs.String("since", "", "Compute changed-symbol set from files changed since this git ref (e.g. HEAD~1, main)")

	positional, rest := splitImpactArgs(args)
	if err := fs.Parse(rest); err != nil {
		return 1
	}

	modes := 0
	if len(positional) > 0 {
		modes++
	}
	if *fromFile != "" {
		modes++
	}
	if *since != "" {
		modes++
	}
	if modes == 0 {
		fmt.Fprintln(os.Stderr, "usage: krit impact <fqn>... | --from-file PATH | --since GITREF [--json]")
		return 1
	}
	if modes > 1 {
		fmt.Fprintln(os.Stderr, "error: pass exactly one of <fqn>..., --from-file, --since")
		return 1
	}

	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	changed := positional
	if *fromFile != "" {
		fqns, err := changedFQNsFromFile(*fromFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		changed = fqns
	}
	if *since != "" {
		fqns, err := changedFQNsFromGit(root, *since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		changed = fqns
	}
	if len(changed) == 0 {
		// Nothing changed; emit an empty result rather than failing.
		return emitImpact(*jsonFlag, root, nil, nil, nil)
	}

	files, err := scanProjectKotlinFiles(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	idx := scanner.BuildIndex(files, runtime.NumCPU())

	hits := computeImpact(idx, changed)
	graph, _ := module.DiscoverModules(root)
	return emitImpact(*jsonFlag, root, changed, hits, graph)
}

// splitImpactArgs splits args into positional FQNs (anything not
// starting with '-') and flag args. Order is preserved.
func splitImpactArgs(args []string) (positional, rest []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			rest = append(rest, a)
			// Flags with values consume the next arg unless joined with '='.
			if !strings.Contains(a, "=") && (a == "--from-file" || a == "-from-file" || a == "--since" || a == "-since") {
				if i+1 < len(args) {
					i++
					rest = append(rest, args[i])
				}
			}
			continue
		}
		positional = append(positional, a)
	}
	return positional, rest
}

// simpleName extracts the last dot-separated segment of an FQN, ignoring
// any parenthesised arity/signature discriminator.
func simpleName(fqn string) string {
	if i := strings.IndexByte(fqn, '('); i >= 0 {
		fqn = fqn[:i]
	}
	if i := strings.LastIndexByte(fqn, '.'); i >= 0 {
		return fqn[i+1:]
	}
	return fqn
}

// computeImpact returns hit files mapped to the set of changed symbol
// FQNs that reach them. Iterates to a fixed point so transitively
// affected files (callers of callers) are included.
func computeImpact(idx *scanner.CodeIndex, changed []string) map[string]map[string]bool {
	declsByName := make(map[string]map[string]bool)
	declsByFile := make(map[string][]string)
	for _, sym := range idx.Symbols {
		if declsByName[sym.Name] == nil {
			declsByName[sym.Name] = make(map[string]bool)
		}
		declsByName[sym.Name][sym.File] = true
		if sym.Visibility != "private" {
			declsByFile[sym.File] = append(declsByFile[sym.File], sym.Name)
		}
	}

	hits := make(map[string]map[string]bool)
	worklist := append([]string{}, changed...)
	processed := make(map[string]bool)

	for len(worklist) > 0 {
		fqn := worklist[0]
		worklist = worklist[1:]
		if processed[fqn] {
			continue
		}
		processed[fqn] = true

		name := simpleName(fqn)
		if name == "" {
			continue
		}
		decls := declsByName[name]
		for f := range idx.ReferenceFiles(name) {
			if decls[f] {
				continue
			}
			if hits[f] == nil {
				hits[f] = make(map[string]bool)
				for _, declName := range declsByFile[f] {
					if !processed[declName] {
						worklist = append(worklist, declName)
					}
				}
			}
			hits[f][fqn] = true
		}
	}
	return hits
}

func emitImpact(asJSON bool, root string, changed []string, hits map[string]map[string]bool, graph *module.ModuleGraph) int {
	files := make([]string, 0, len(hits))
	for f := range hits {
		files = append(files, f)
	}
	sort.Strings(files)

	if asJSON {
		out := impactJSON{Changed: changed, Impact: make([]impactJSONEntry, 0, len(files))}
		for _, f := range files {
			syms := make([]string, 0, len(hits[f]))
			for s := range hits[f] {
				syms = append(syms, s)
			}
			sort.Strings(syms)
			entry := impactJSONEntry{File: relPath(root, f), Symbols: syms}
			if graph != nil {
				entry.Module = graph.FileToModule(f)
			}
			out.Impact = append(out.Impact, entry)
		}
		if out.Changed == nil {
			out.Changed = []string{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	modules := make(map[string]bool)
	for _, f := range files {
		fmt.Println(relPath(root, f))
		if graph != nil {
			if m := graph.FileToModule(f); m != "" {
				modules[m] = true
			}
		}
	}
	if len(files) > 0 {
		if len(modules) > 0 {
			fmt.Printf("  (%d files across %d modules)\n", len(files), len(modules))
		} else {
			fmt.Printf("  (%d files)\n", len(files))
		}
	}
	return 0
}

func relPath(root, p string) string {
	if rel, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return p
}

// changedFQNsFromFile returns the FQNs declared in a single Kotlin file.
func changedFQNsFromFile(path string) ([]string, error) {
	f, err := scanner.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	sigs := arch.ExtractAbiSignatures([]*scanner.File{f})
	out := make([]string, 0, len(sigs))
	seen := make(map[string]bool)
	for _, s := range sigs {
		if seen[s.FQN] {
			continue
		}
		seen[s.FQN] = true
		out = append(out, s.FQN)
	}
	return out, nil
}

// changedFQNsFromGit lists Kotlin files changed since gitRef (via
// `git diff --name-only`) and returns the union of their declared FQNs.
func changedFQNsFromGit(root, gitRef string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", gitRef)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", gitRef, err)
	}
	var fqns []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasSuffix(line, ".kt") {
			continue
		}
		full := filepath.Join(root, line)
		if _, err := os.Stat(full); err != nil {
			continue
		}
		fileFqns, err := changedFQNsFromFile(full)
		if err != nil {
			continue
		}
		for _, fqn := range fileFqns {
			if seen[fqn] {
				continue
			}
			seen[fqn] = true
			fqns = append(fqns, fqn)
		}
	}
	return fqns, nil
}

// scanProjectKotlinFiles parses every .kt file rooted at root.
func scanProjectKotlinFiles(root string) ([]*scanner.File, error) {
	paths, err := scanner.CollectKotlinFiles([]string{root}, nil)
	if err != nil {
		return nil, fmt.Errorf("collecting Kotlin files: %w", err)
	}
	files, _ := scanner.ScanFiles(paths, runtime.NumCPU())
	return files, nil
}
