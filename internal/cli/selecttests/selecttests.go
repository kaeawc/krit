package selecttests

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/proc"
	"github.com/kaeawc/krit/internal/scanner"
)

func Run(args []string) int {
	fs := flag.NewFlagSet("select-tests", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	base := fs.String("base", "", "Git ref to diff against")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *base == "" {
		fmt.Fprintln(os.Stderr, "usage: krit select-tests --base <ref>")
		return 1
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	changed, err := changedKotlinFiles(root, *base)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	files, err := scanKotlinFiles(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	for _, path := range SelectTests(root, files, changed) {
		fmt.Println(path)
	}
	return 0
}

func SelectTests(root string, files []*scanner.File, changedPaths []string) []string {
	idx := scanner.BuildIndex(files, runtime.NumCPU())
	changedSet := changedPathSet(root, changedPaths)
	changedNames := make(map[string]bool)
	for _, sym := range idx.Symbols {
		if sym.Language != scanner.LangKotlin || isTestPath(sym.File) {
			continue
		}
		if !changedSet[filepath.Clean(sym.File)] {
			continue
		}
		if sym.Name != "" {
			changedNames[sym.Name] = true
		}
		if sym.FQN != "" {
			changedNames[sym.FQN] = true
		}
	}
	selected := make(map[string]bool)
	for name := range changedNames {
		for path := range idx.ReferenceFiles(name) {
			if isTestPath(path) {
				selected[relPath(root, path)] = true
			}
		}
	}
	out := make([]string, 0, len(selected))
	for path := range selected {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func changedKotlinFiles(root, base string) ([]string, error) {
	return changedKotlinFilesWith(proc.Default, root, base)
}

func changedKotlinFilesWith(runner proc.Runner, root, base string) ([]string, error) {
	res, err := runner.Run(context.Background(), proc.Cmd{
		Name: "git",
		Args: []string{"diff", "--name-only", base},
		Dir:  root,
	})
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", base, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("git diff %s: exit %d: %s", base, res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(res.Stdout)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(line, ".kt") {
			continue
		}
		out = append(out, filepath.Join(root, filepath.FromSlash(line)))
	}
	return out, nil
}

func changedPathSet(root string, paths []string) map[string]bool {
	out := make(map[string]bool, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, filepath.FromSlash(path))
		}
		out[filepath.Clean(path)] = true
	}
	return out
}

func scanKotlinFiles(root string) ([]*scanner.File, error) {
	var out []*scanner.File
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", ".gradle", ".idea", ".kotlin", "build", "out", "target", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".kt") {
			return nil
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err == nil {
			out = append(out, file)
		}
		return nil
	})
	return out, err
}

func isTestPath(path string) bool {
	slash := filepath.ToSlash(path)
	for _, marker := range []string{"/test/", "/androidTest/", "/commonTest/", "/jvmTest/", "/testFixtures/"} {
		if strings.Contains(slash, marker) {
			return true
		}
	}
	return false
}

func relPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}
