package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"

	krename "github.com/kaeawc/krit/internal/rename"
	"github.com/kaeawc/krit/internal/scanner"
)

type renameCommand struct {
	FromFQN string
	ToFQN   string
	Paths   []string
	Jobs    int
}

func runRenameSubcommand(args []string) int {
	cmd := renameCommand{}
	fs := flag.NewFlagSet("rename", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.IntVar(&cmd.Jobs, "j", runtime.NumCPU(), "Number of parallel jobs")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: krit rename [flags] <from-fqn> <to-fqn> [paths...]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	positional := fs.Args()
	if len(positional) < 2 {
		fs.Usage()
		return 2
	}
	cmd.FromFQN = positional[0]
	cmd.ToFQN = positional[1]
	if len(positional) > 2 {
		cmd.Paths = positional[2:]
	} else {
		cmd.Paths = []string{"."}
	}

	return runRenameCommand(cmd)
}

func runRenameCommand(cmd renameCommand) int {
	target, err := krename.ParseTarget(cmd.FromFQN, cmd.ToFQN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	if cmd.Jobs <= 0 {
		cmd.Jobs = 1
	}

	plan, err := buildRenamePlan(cmd.Paths, cmd.Jobs, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: rename: %v\n", err)
		return 2
	}

	summary := plan.Summary()
	fmt.Printf(
		"Rename planning found %d candidate occurrences in %d files (%d declarations, %d references) for %s -> %s.\n",
		plan.CandidateCount(),
		summary.Files,
		summary.Declarations,
		summary.References,
		target.FromFQN,
		target.ToFQN,
	)
	fmt.Fprintf(os.Stderr, "error: %v\n", krename.ErrApplyNotImplemented)
	return 2
}

func buildRenamePlan(paths []string, workers int, target krename.Target) (krename.Plan, error) {
	kotlinPaths, err := scanner.CollectKotlinFiles(paths, nil)
	if err != nil {
		return krename.Plan{}, err
	}
	parsedFiles, scanErrs := scanner.ScanFiles(kotlinPaths, workers)
	if err := joinErrors(scanErrs); err != nil {
		return krename.Plan{}, err
	}

	javaPaths, err := scanner.CollectJavaFiles(paths, nil)
	if err != nil {
		return krename.Plan{}, err
	}
	parsedJavaFiles, javaErrs := scanner.ScanJavaFiles(javaPaths, workers)
	if err := joinErrors(javaErrs); err != nil {
		return krename.Plan{}, err
	}

	idx := scanner.BuildIndex(parsedFiles, workers, parsedJavaFiles...)
	return krename.BuildPlan(idx, target), nil
}

func joinErrors(errs []error) error {
	var filtered []error
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return errors.Join(filtered...)
}
