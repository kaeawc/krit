package digraph

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/kaeawc/krit/internal/di"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

func Run(args []string) int {
	fs := flag.NewFlagSet("di-graph", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	format := fs.String("format", "json", "Output format: json, dot, or mermaid")
	moduleFilter := fs.String("module", "", "Gradle module path to include, for example :app")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	normalized := di.NormalizeExportFormat(*format)
	if normalized == "" {
		fmt.Fprintf(os.Stderr, "error: unsupported format %q; use json, dot, or mermaid\n", *format)
		return 1
	}
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	moduleGraph, err := module.DiscoverModules(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: discover modules: %v\n", err)
		return 1
	}
	files, err := scanner.CollectKotlinFiles(paths, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: collect files: %v\n", err)
		return 1
	}
	parsed, errs := scanner.ScanFiles(context.Background(), files, runtime.NumCPU())
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "error: parse: %v\n", errs[0])
		return 1
	}
	graph := di.BuildGraph(parsed, moduleGraph)
	switch normalized {
	case "json":
		err = graph.ExportJSON(os.Stdout, *moduleFilter)
	case "dot":
		err = graph.ExportDOT(os.Stdout, *moduleFilter)
	case "mermaid":
		err = graph.ExportMermaid(os.Stdout, *moduleFilter)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: export: %v\n", err)
		return 1
	}
	return 0
}
