// Command krit-docgen writes the per-rule documentation pages under
// docs/rules/. Run it from the repo root before `mkdocs build`:
//
//	go run ./cmd/krit-docgen
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/config"
	"github.com/kaeawc/krit/internal/docgen"
	_ "github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/api"
)

func main() {
	root := flag.String("root", ".", "repo root")
	out := flag.String("out", "", "output directory (default <root>/docs/rules)")
	flag.Parse()
	if *out == "" {
		*out = filepath.Join(*root, "docs", "rules")
	}
	migration, err := os.ReadFile(filepath.Join(*root, "docs", "detekt-migration.md"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "krit-docgen:", err)
		os.Exit(1)
	}
	src := docgen.Sources{
		Fixtures:       os.DirFS(filepath.Join(*root, "tests", "fixtures")),
		DetektDefaults: config.DetektDefaults(),
		Migration:      string(migration),
	}
	if err := docgen.Generate(api.Registry, src, *out); err != nil {
		fmt.Fprintln(os.Stderr, "krit-docgen:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d rule pages to %s\n", len(api.Registry), *out)
}
