package rename

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"

	krename "github.com/kaeawc/krit/internal/rename"
	"github.com/kaeawc/krit/internal/scanner"
)

type renameCommand struct {
	FromFQN                   string
	ToFQN                     string
	Paths                     []string
	Jobs                      int
	Apply                     bool
	AllowMultipleDeclarations bool
}

func Run(args []string) int {
	cmd := renameCommand{}
	fs := flag.NewFlagSet("rename", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.IntVar(&cmd.Jobs, "j", runtime.NumCPU(), "Number of parallel jobs")
	fs.BoolVar(&cmd.Apply, "apply", false, "Apply the rename to the working tree (default: dry-run)")
	fs.BoolVar(&cmd.AllowMultipleDeclarations, "allow-multiple-declarations", false, "Bypass the multi-decl ambiguity guard (e.g. for legitimate Kotlin overload sets sharing an FQN)")
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

	if err := krename.ValidatePlan(plan, krename.ValidateOptions{
		AllowMultipleDeclarations: cmd.AllowMultipleDeclarations,
	}); err != nil {
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

	if !cmd.Apply {
		res, err := krename.DryRunApply(plan)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: rename dry-run: %v\n", err)
			return 2
		}
		fmt.Printf("Dry run: would change %d files (%d edits)", res.FilesChanged, res.Edits)
		if len(res.Moves) > 0 {
			fmt.Printf(", %d file moves", len(res.Moves))
		}
		fmt.Println(". Re-run with --apply to write changes.")
		for _, mv := range res.Moves {
			fmt.Printf("  move: %s -> %s\n", mv.From, mv.To)
		}
		return 0
	}

	res, err := krename.Apply(plan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: rename apply: %v\n", err)
		return 1
	}
	fmt.Printf("Applied: %d files changed (%d edits)", res.FilesChanged, res.Edits)
	if len(res.Moves) > 0 {
		fmt.Printf(", %d file moves", len(res.Moves))
	}
	fmt.Println(".")
	for _, mv := range res.Moves {
		fmt.Printf("  moved: %s -> %s\n", mv.From, mv.To)
	}
	return 0
}

func buildRenamePlan(paths []string, workers int, target krename.Target) (krename.Plan, error) {
	kotlinPaths, err := scanner.CollectKotlinFiles(paths, nil)
	if err != nil {
		return krename.Plan{}, err
	}
	parsedFiles, scanErrs := scanner.ScanFiles(context.Background(), kotlinPaths, workers)
	if err := joinErrors(scanErrs); err != nil {
		return krename.Plan{}, err
	}

	javaPaths, err := scanner.CollectJavaFiles(paths, nil)
	if err != nil {
		return krename.Plan{}, err
	}
	parsedJavaFiles, javaErrs := scanner.ScanJavaFiles(context.Background(), javaPaths, workers)
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
