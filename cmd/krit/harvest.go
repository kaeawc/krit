package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kaeawc/krit/internal/harvest"
)

type harvestCommand struct {
	Target string
	Rule   string
	Out    string
}

func runHarvestSubcommand(args []string) int {
	cmd := harvestCommand{}
	fs := flag.NewFlagSet("harvest", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cmd.Rule, "rule", "", "Rule name for the finding to extract")
	fs.StringVar(&cmd.Out, "out", "", "Output fixture path")

	filteredArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if cmd.Target == "" && !strings.HasPrefix(arg, "-") {
			cmd.Target = arg
			continue
		}
		filteredArgs = append(filteredArgs, arg)
	}
	if err := fs.Parse(filteredArgs); err != nil {
		return 2
	}
	if cmd.Target == "" && len(fs.Args()) > 0 {
		cmd.Target = fs.Args()[0]
	}
	return runHarvestCommand(cmd)
}

func runHarvestCommand(cmd harvestCommand) int {
	if cmd.Target == "" {
		fmt.Fprintln(os.Stderr, "error: harvest requires SOURCE:LINE")
		return 2
	}
	if cmd.Rule == "" {
		fmt.Fprintln(os.Stderr, "error: harvest requires --rule")
		return 2
	}
	if cmd.Out == "" {
		fmt.Fprintln(os.Stderr, "error: harvest requires --out")
		return 2
	}

	target, err := harvest.ParseTarget(cmd.Target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: harvest target: %v\n", err)
		return 2
	}

	result, err := harvest.ExtractFixture(target, cmd.Rule)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: harvest: %v\n", err)
		return 2
	}
	if err := harvest.WriteFixture(cmd.Out, result); err != nil {
		fmt.Fprintf(os.Stderr, "error: harvest write: %v\n", err)
		return 2
	}

	fmt.Printf("Harvested %s from %s:%d to %s (%s lines %d-%d)\n",
		result.Finding.Rule,
		target.Path,
		result.Finding.Line,
		cmd.Out,
		result.NodeType,
		result.StartLine,
		result.EndLine,
	)
	return 0
}
