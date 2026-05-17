package transform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/codemod"
)

func Run(args []string) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintln(os.Stderr, "usage: krit transform <recipe.yml|recipe-name> [--apply|--dry-run] [--root DIR]")
		return 1
	}
	root := opts.root
	if root == "" {
		root, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	}
	recipePath := resolveRecipePath(root, opts.recipe)
	recipe, err := codemod.LoadRecipe(recipePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	result, err := codemod.Run(context.Background(), root, recipe, opts.apply)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if opts.apply {
		fmt.Printf("matched %d sites in %d files; wrote %d edits in %d files\n",
			result.Matches, result.FilesMatched, result.EditsApplied, result.FilesModified)
	} else {
		fmt.Printf("matched %d sites in %d files; dry-run only\n", result.Matches, result.FilesMatched)
		for _, edit := range result.Edits {
			if rel, err := filepath.Rel(root, edit.File); err == nil {
				fmt.Printf("  %s:%d-%d\n", filepath.ToSlash(rel), edit.StartByte, edit.EndByte)
			}
		}
	}
	return 0
}

type options struct {
	recipe string
	root   string
	apply  bool
}

func parseArgs(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--apply":
			opts.apply = true
		case "--dry-run":
			opts.apply = false
		case "--root":
			if i+1 >= len(args) {
				return options{}, fmt.Errorf("--root requires a value")
			}
			i++
			opts.root = args[i]
		default:
			if len(arg) > 7 && arg[:7] == "--root=" {
				opts.root = arg[7:]
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return options{}, fmt.Errorf("unknown flag %s", arg)
			}
			if opts.recipe != "" {
				return options{}, fmt.Errorf("expected one recipe")
			}
			opts.recipe = arg
		}
	}
	if opts.recipe == "" {
		return options{}, fmt.Errorf("expected recipe")
	}
	return opts, nil
}

func resolveRecipePath(root, arg string) string {
	if filepath.Ext(arg) == ".yml" || filepath.Ext(arg) == ".yaml" || filepath.IsAbs(arg) {
		return arg
	}
	return filepath.Join(root, "recipes", arg+".yml")
}
