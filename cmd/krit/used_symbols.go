package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/usedsymbols"
)

type usedSymbolsModuleResult struct {
	Module  string              `json:"module"`
	Symbols []usedsymbols.Symbol `json:"symbols"`
}

// runUsedSymbolsSubcommand implements `krit used-symbols :module` and
// `krit used-symbols path/to/File.kt`. Output is one symbol per line in
// plain mode, JSON with `--json`. Same-module symbols are filtered out
// because they don't cross the build cache-key boundary the consumer
// of this command cares about.
func runUsedSymbolsSubcommand(args []string) int {
	fs := flag.NewFlagSet("used-symbols", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonFlag := fs.Bool("json", false, "Emit JSON instead of plain text")

	positional, rest := splitPositional(args, 1)
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if len(positional) == 0 {
		fmt.Fprintln(os.Stderr, "usage: krit used-symbols <:module|path/to/File.kt> [--json]")
		return 1
	}
	target := positional[0]

	if strings.HasPrefix(target, ":") {
		return runUsedSymbolsModule(target, *jsonFlag)
	}
	return runUsedSymbolsFile(target, *jsonFlag)
}

func runUsedSymbolsModule(modulePath string, asJSON bool) int {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	graph, err := module.DiscoverModules(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "discovering modules: %v\n", err)
		return 1
	}
	mod, ok := graph.Modules[modulePath]
	if !ok {
		fmt.Fprintf(os.Stderr, "module %q not found\n", modulePath)
		return 1
	}
	files := scanModuleKotlinFiles(mod)
	pkgs := collectPackages(files)
	syms := aggregateSymbols(files, func(fqn string) bool {
		// Walk leftward through fqn's prefixes, checking each against pkgs.
		// O(segments) lookups vs. O(len(pkgs)) prefix scans.
		for s := fqn; s != ""; {
			if _, ok := pkgs[s]; ok {
				return true
			}
			i := strings.LastIndex(s, ".")
			if i < 0 {
				return false
			}
			s = s[:i]
		}
		return false
	})

	if asJSON {
		res := usedSymbolsModuleResult{Module: modulePath, Symbols: syms}
		if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Println(modulePath)
	for _, s := range syms {
		fmt.Printf("  %s\n", s.FQN)
	}
	return 0
}

func runUsedSymbolsFile(path string, asJSON bool) int {
	if !strings.HasSuffix(path, ".kt") {
		fmt.Fprintf(os.Stderr, "expected a .kt file or :module path, got %s\n", path)
		return 1
	}
	f, err := scanner.ParseFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parsing %s: %v\n", path, err)
		return 1
	}
	syms := usedsymbols.Extract(f)

	if asJSON {
		res := usedsymbols.FileResult{File: path, Symbols: syms}
		if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Println(path)
	for _, s := range syms {
		fmt.Printf("  %s\n", s.FQN)
	}
	return 0
}

// aggregateSymbols extracts and deduplicates symbols across files,
// dropping any whose FQN passes the in-module predicate.
func aggregateSymbols(files []*scanner.File, isInModule func(string) bool) []usedsymbols.Symbol {
	seen := make(map[string]usedsymbols.Symbol)
	for _, f := range files {
		for _, s := range usedsymbols.Extract(f) {
			if isInModule != nil && isInModule(s.FQN) {
				continue
			}
			key := s.Kind + "|" + s.FQN
			if existing, ok := seen[key]; ok {
				if s.Arity > existing.Arity {
					existing.Arity = s.Arity
					seen[key] = existing
				}
				continue
			}
			seen[key] = s
		}
	}
	out := make([]usedsymbols.Symbol, 0, len(seen))
	for _, s := range seen {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FQN != out[j].FQN {
			return out[i].FQN < out[j].FQN
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// collectPackages returns the set of package names declared across the
// given Kotlin source files, used to recognize same-module references.
func collectPackages(files []*scanner.File) map[string]struct{} {
	pkgs := make(map[string]struct{})
	for _, f := range files {
		if pkg := usedsymbols.Package(f); pkg != "" {
			pkgs[pkg] = struct{}{}
		}
	}
	return pkgs
}
