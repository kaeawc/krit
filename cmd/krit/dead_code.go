package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kaeawc/krit/internal/deadcode"
)

// runDeadCodeSubcommand implements `krit dead-code [--project] [--json] [--root FQN]... [paths...]`.
func runDeadCodeSubcommand(args []string) int {
	fs := flag.NewFlagSet("dead-code", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	projectFlag := fs.Bool("project", false, "Run project-level cross-module reachability analysis")
	jsonFlag := fs.Bool("json", false, "Emit findings as JSON")
	var roots multiStringFlag
	fs.Var(&roots, "root", "Additional reachability root (FQN or simple name); may repeat")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if !*projectFlag {
		fmt.Fprintln(os.Stderr, "usage: krit dead-code --project [--json] [--root FQN]... [paths...]")
		fmt.Fprintln(os.Stderr, "       (file-level dead code is reported by the regular `krit` scan via the DeadCode rule)")
		return 1
	}

	scanRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	findings, err := deadcode.AnalyzeProject(scanRoot, deadcode.ProjectOptions{
		Roots: roots,
		Paths: fs.Args(),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(findings); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	for _, f := range findings {
		signature := formatProjectSignature(f)
		fmt.Printf("%s:%d\n  %s  [%s]\n", f.File, f.Line, signature, f.Reason)
	}
	return 0
}

func formatProjectSignature(f deadcode.ProjectFinding) string {
	vis := f.Visibility
	if vis == "" {
		vis = "public"
	}
	return fmt.Sprintf("%s %s %s", vis, f.Kind, f.Name)
}

// multiStringFlag accumulates repeated string flag values.
type multiStringFlag []string

func (m *multiStringFlag) String() string { return fmt.Sprintf("%v", []string(*m)) }

func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}
