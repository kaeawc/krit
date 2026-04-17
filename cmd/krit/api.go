package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

// resolveModuleSurface discovers modules from cwd, scans the named module's
// Kotlin files, and returns the extracted API surface.
func resolveModuleSurface(modulePath string) ([]arch.APIEntry, error) {
	scanRoot, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	graph, err := module.DiscoverModules(scanRoot)
	if err != nil {
		return nil, fmt.Errorf("discovering modules: %w", err)
	}
	mod, ok := graph.Modules[modulePath]
	if !ok {
		return nil, fmt.Errorf("module %q not found", modulePath)
	}
	files := scanModuleKotlinFiles(mod)
	idx := scanner.BuildIndex(files, runtime.NumCPU())
	return arch.ExtractSurface(idx.Symbols), nil
}

// runAPISnapshotSubcommand implements `krit api-snapshot :module > surface.txt`.
func runAPISnapshotSubcommand(args []string) int {
	fs := flag.NewFlagSet("api-snapshot", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	outFlag := fs.String("o", "", "Write output to file (default: stdout)")

	modulePath, rest := splitPositional(args, 1)
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if len(modulePath) == 0 {
		fmt.Fprintln(os.Stderr, "usage: krit api-snapshot :module [-o output.txt]")
		return 1
	}

	entries, err := resolveModuleSurface(modulePath[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	output := arch.FormatSurface(entries)

	if *outFlag != "" {
		if err := os.WriteFile(*outFlag, []byte(output), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", *outFlag, err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "wrote %d API entries to %s\n", len(entries), *outFlag)
	} else {
		fmt.Print(output)
	}
	return 0
}

// runAPIDiffSubcommand implements `krit api-diff :module baseline.txt`.
func runAPIDiffSubcommand(args []string) int {
	fs := flag.NewFlagSet("api-diff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	positional, rest := splitPositional(args, 2)
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if len(positional) < 2 {
		fmt.Fprintln(os.Stderr, "usage: krit api-diff :module baseline.txt")
		return 1
	}
	modulePath, baselineFile := positional[0], positional[1]

	baselineData, err := os.ReadFile(baselineFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading baseline %s: %v\n", baselineFile, err)
		return 1
	}
	oldEntries := arch.ParseSurface(string(baselineData))

	newEntries, err := resolveModuleSurface(modulePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	diffs := arch.DiffSurfaces(oldEntries, newEntries)

	if len(diffs) == 0 {
		fmt.Println("No API changes detected.")
		return 0
	}
	for _, d := range diffs {
		switch d.Change {
		case arch.ChangeAdded:
			fmt.Printf("+ %s\t%s\n", d.Entry.Kind, d.Entry.Name)
		case arch.ChangeRemoved:
			fmt.Printf("- %s\t%s\n", d.Entry.Kind, d.Entry.Name)
		}
	}
	fmt.Fprintf(os.Stderr, "\n%d API change(s) detected.\n", len(diffs))
	return 1
}

// splitPositional pulls up to max leading non-flag arguments out of args and
// returns them alongside the remaining (flag) arguments. Once a flag appears,
// all subsequent arguments are treated as flag args.
func splitPositional(args []string, max int) (positional, rest []string) {
	rest = make([]string, 0, len(args))
	for _, arg := range args {
		if len(positional) < max && !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		rest = append(rest, arg)
	}
	return positional, rest
}

// scanModuleKotlinFiles finds and parses all .kt files under the module's source roots.
func scanModuleKotlinFiles(mod *module.Module) []*scanner.File {
	var ktFiles []string
	roots := mod.SourceRoots
	if len(roots) == 0 {
		roots = []string{filepath.Join(mod.Dir, "src", "main", "kotlin")}
	}
	for _, root := range roots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.HasSuffix(path, ".kt") {
				ktFiles = append(ktFiles, path)
			}
			return nil
		})
	}
	files := make([]*scanner.File, 0, len(ktFiles))
	for _, path := range ktFiles {
		f, err := scanner.ParseFile(path)
		if err != nil {
			continue
		}
		files = append(files, f)
	}
	return files
}
