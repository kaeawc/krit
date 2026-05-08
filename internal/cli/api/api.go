package api

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/cli/clishared"
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
	files := clishared.ScanModuleKotlinFiles(mod)
	idx := scanner.BuildIndex(files, runtime.NumCPU())
	return arch.ExtractSurface(idx.Symbols), nil
}

// runAPISnapshotSubcommand implements `krit api-snapshot :module > surface.txt`.
func RunSnapshot(args []string) int {
	fs := flag.NewFlagSet("api-snapshot", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	outFlag := fs.String("o", "", "Write output to file (default: stdout)")

	modulePath, rest := clishared.SplitPositional(args, 1)
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
func RunDiff(args []string) int {
	fs := flag.NewFlagSet("api-diff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	positional, rest := clishared.SplitPositional(args, 2)
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
