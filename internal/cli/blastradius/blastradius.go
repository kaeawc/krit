package blastradius

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

type SymbolImpact struct {
	Symbol        string   `json:"symbol"`
	File          string   `json:"file"`
	Line          int      `json:"line"`
	ConsumerFiles int      `json:"consumerFiles"`
	Modules       []string `json:"modules"`
}

type Report struct {
	Base            string         `json:"base"`
	ChangedFiles    int            `json:"changedFiles"`
	ChangedSymbols  int            `json:"changedSymbols"`
	DirectConsumers int            `json:"directConsumers"`
	Modules         int            `json:"modules"`
	TopFanIn        []SymbolImpact `json:"topFanIn"`
}

func Run(args []string) int {
	fs := flag.NewFlagSet("blast-radius", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	base := fs.String("base", "main", "Git base ref")
	format := fs.String("format", "plain", "Output format: plain or json")
	root := fs.String("root", "", "Repository root. Defaults to current directory.")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	wd := *root
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	}
	report, err := Analyze(wd, *base)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	case "plain":
		fmt.Printf("Changed files: %d\n", report.ChangedFiles)
		fmt.Printf("Changed symbols: %d\n", report.ChangedSymbols)
		fmt.Printf("Direct consumers: %d files across %d modules\n", report.DirectConsumers, report.Modules)
		if len(report.TopFanIn) > 0 {
			fmt.Println("Top fan-in changes:")
			for _, item := range report.TopFanIn {
				fmt.Printf("  %s: %d consumers in %d modules\n", item.Symbol, item.ConsumerFiles, len(item.Modules))
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "error: unknown format %q\n", *format)
		return 1
	}
	return 0
}

func Analyze(root, base string) (Report, error) {
	changedLines, err := changedLineIntervals(root, base)
	if err != nil {
		return Report{}, err
	}
	paths, err := scanner.CollectKotlinFiles([]string{root}, nil)
	if err != nil {
		return Report{}, err
	}
	files, errs := scanner.ScanFiles(context.Background(), paths, runtime.NumCPU())
	if len(errs) > 0 {
		return Report{}, errs[0]
	}
	index := scanner.BuildIndex(files, runtime.NumCPU())
	graph, _ := module.DiscoverModules(root)
	if graph == nil {
		graph = module.NewModuleGraph(root)
	}

	consumerUnion := make(map[string]bool)
	moduleUnion := make(map[string]bool)
	var impacts []SymbolImpact
	for _, sym := range index.Symbols {
		intervals := changedLines[absPath(sym.File)]
		if !lineInIntervals(sym.Line, intervals) {
			continue
		}
		consumers := consumerFiles(index, sym)
		delete(consumers, sym.File)
		modules := modulesForFiles(graph, consumers)
		for file := range consumers {
			consumerUnion[file] = true
		}
		for _, mod := range modules {
			moduleUnion[mod] = true
		}
		impacts = append(impacts, SymbolImpact{
			Symbol:        symbolName(sym),
			File:          relPath(root, sym.File),
			Line:          sym.Line,
			ConsumerFiles: len(consumers),
			Modules:       modules,
		})
	}
	sort.Slice(impacts, func(i, j int) bool {
		if impacts[i].ConsumerFiles != impacts[j].ConsumerFiles {
			return impacts[i].ConsumerFiles > impacts[j].ConsumerFiles
		}
		return impacts[i].Symbol < impacts[j].Symbol
	})
	if len(impacts) > 20 {
		impacts = impacts[:20]
	}
	return Report{
		Base:            base,
		ChangedFiles:    len(changedLines),
		ChangedSymbols:  len(impacts),
		DirectConsumers: len(consumerUnion),
		Modules:         len(moduleUnion),
		TopFanIn:        impacts,
	}, nil
}

func consumerFiles(index *scanner.CodeIndex, sym scanner.Symbol) map[string]bool {
	out := make(map[string]bool)
	for _, name := range []string{sym.Name, sym.FQN} {
		if name == "" {
			continue
		}
		for file := range index.ReferenceFiles(name) {
			out[file] = true
		}
	}
	return out
}

func modulesForFiles(graph *module.Graph, files map[string]bool) []string {
	set := make(map[string]bool)
	for file := range files {
		mod := graph.FileToModule(file)
		if mod == "" {
			mod = ":"
		}
		set[mod] = true
	}
	mods := make([]string, 0, len(set))
	for mod := range set {
		mods = append(mods, mod)
	}
	sort.Strings(mods)
	return mods
}

func symbolName(sym scanner.Symbol) string {
	if sym.FQN != "" {
		return sym.FQN
	}
	if sym.Owner != "" {
		return sym.Owner + "." + sym.Name
	}
	return sym.Name
}

type interval struct{ start, end int }

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func changedLineIntervals(root, base string) (map[string][]interval, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", root, "diff", "--unified=0", "--diff-filter=ACMR", base, "--", "*.kt", "*.kts")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", base, err)
	}
	result := make(map[string][]interval)
	var file string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "+++ ") {
			path := strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(line, "+++ ")), "b/")
			if path == "/dev/null" {
				file = ""
			} else {
				file = absPath(filepath.Join(root, path))
			}
			continue
		}
		m := hunkRe.FindStringSubmatch(line)
		if m == nil || file == "" {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		if count > 0 {
			result[file] = append(result[file], interval{start: start, end: start + count - 1})
		}
	}
	return result, nil
}

func lineInIntervals(line int, intervals []interval) bool {
	for _, interval := range intervals {
		if line >= interval.start && line <= interval.end {
			return true
		}
	}
	return false
}

func absPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(abs)
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
