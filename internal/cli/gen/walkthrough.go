package gen

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

func runWalkthrough(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("walkthrough", flag.ContinueOnError)
	fs.SetOutput(stderr)
	limit := fs.Int("n", 10, "Maximum files to include")
	report := fs.String("report", "plain", "Report format: plain or json")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(stderr, "walkthrough: resolving %s: %v\n", root, err)
		return 1
	}
	walkthrough, err := BuildWalkthrough(absRoot, *limit)
	if err != nil {
		fmt.Fprintf(stderr, "walkthrough: %v\n", err)
		return 1
	}
	output, err := RenderWalkthrough(walkthrough, *report)
	if err != nil {
		fmt.Fprintf(stderr, "walkthrough: %v\n", err)
		return 1
	}
	if _, err := io.WriteString(stdout, output); err != nil {
		fmt.Fprintf(stderr, "walkthrough: writing report: %v\n", err)
		return 1
	}
	return 0
}

type Walkthrough struct {
	Seed  WalkthroughSeed   `json:"seed"`
	Files []WalkthroughFile `json:"files"`
}

type WalkthroughSeed struct {
	Symbol string `json:"symbol"`
	File   string `json:"file"`
	Why    string `json:"why"`
	FanIn  int    `json:"fanIn"`
}

type WalkthroughFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	Score  int    `json:"score"`
}

func BuildWalkthrough(root string, limit int) (Walkthrough, error) {
	if limit < 1 {
		limit = 1
	}
	kotlinPaths, err := scanner.CollectKotlinFiles([]string{root}, nil)
	if err != nil {
		return Walkthrough{}, fmt.Errorf("collecting Kotlin files: %w", err)
	}
	javaPaths, err := scanner.CollectJavaFiles([]string{root}, nil)
	if err != nil {
		return Walkthrough{}, fmt.Errorf("collecting Java files: %w", err)
	}
	kotlinFiles, parseErrs := scanner.ScanFiles(context.Background(), kotlinPaths, runtime.NumCPU())
	if len(parseErrs) > 0 {
		return Walkthrough{}, fmt.Errorf("parsing Kotlin files: %w", parseErrs[0])
	}
	javaFiles, javaErrs := scanner.ScanJavaFiles(context.Background(), javaPaths, runtime.NumCPU())
	if len(javaErrs) > 0 {
		return Walkthrough{}, fmt.Errorf("parsing Java files: %w", javaErrs[0])
	}
	index := scanner.BuildIndexWithTracker(kotlinFiles, runtime.NumCPU(), nil, javaFiles...)
	stats := index.ClassLikeFanInStats(true)
	if len(stats) == 0 {
		return Walkthrough{}, fmt.Errorf("no class-like symbols found")
	}
	seed := stats[0]
	seedName := symbolDisplayName(seed.Symbol)
	seedPath := relativePath(root, seed.Symbol.File)
	out := Walkthrough{
		Seed: WalkthroughSeed{
			Symbol: seedName,
			File:   seedPath,
			Why:    fmt.Sprintf("highest class-like fan-in (%d external files)", seed.FanIn),
			FanIn:  seed.FanIn,
		},
		Files: []WalkthroughFile{{
			Path:   seedPath,
			Reason: fmt.Sprintf("public API referenced from %d files", seed.FanIn),
			Score:  seed.FanIn,
		}},
	}
	seenFiles := map[string]bool{seed.Symbol.File: true}
	for _, candidate := range collaboratorCandidates(index, seed.Symbol.File) {
		if len(out.Files) >= limit {
			break
		}
		if seenFiles[candidate.file] {
			continue
		}
		seenFiles[candidate.file] = true
		out.Files = append(out.Files, WalkthroughFile{
			Path:   relativePath(root, candidate.file),
			Reason: fmt.Sprintf("collaborator referenced from %d seed call sites", candidate.score),
			Score:  candidate.score,
		})
	}
	return out, nil
}

type collaboratorCandidate struct {
	file  string
	score int
	name  string
}

func collaboratorCandidates(index *scanner.CodeIndex, seedFile string) []collaboratorCandidate {
	byFile := make(map[string]collaboratorCandidate)
	for _, sym := range index.Symbols {
		if sym.File == seedFile || sym.Visibility != "public" {
			continue
		}
		score := countSeedReferences(index, sym, seedFile)
		if score == 0 {
			continue
		}
		current := byFile[sym.File]
		current.file = sym.File
		current.score += score
		if current.name == "" || sym.Name < current.name {
			current.name = sym.Name
		}
		byFile[sym.File] = current
	}
	out := make([]collaboratorCandidate, 0, len(byFile))
	for _, candidate := range byFile {
		out = append(out, candidate)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		if out[i].name != out[j].name {
			return out[i].name < out[j].name
		}
		return out[i].file < out[j].file
	})
	return out
}

func countSeedReferences(index *scanner.CodeIndex, sym scanner.Symbol, seedFile string) int {
	names := []string{sym.Name}
	if sym.FQN != "" && sym.FQN != sym.Name {
		names = append(names, sym.FQN)
	}
	count := 0
	for _, name := range names {
		count += index.CountNonCommentRefsInFile(name, seedFile)
	}
	return count
}

func RenderWalkthrough(w Walkthrough, report string) (string, error) {
	switch report {
	case "json":
		data, err := json.MarshalIndent(w, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data) + "\n", nil
	case "plain":
		var b strings.Builder
		fmt.Fprintf(&b, "Seed: %s (%s)\n", w.Seed.Symbol, w.Seed.File)
		fmt.Fprintf(&b, "Why this file: %s\n\n", w.Seed.Why)
		b.WriteString("Reading order:\n")
		for i, file := range w.Files {
			fmt.Fprintf(&b, "%d. %s\n", i+1, file.Path)
			fmt.Fprintf(&b, "   %s\n", file.Reason)
		}
		return b.String(), nil
	default:
		return "", fmt.Errorf("unknown report %q; use plain or json", report)
	}
}

func symbolDisplayName(sym scanner.Symbol) string {
	if sym.FQN != "" {
		return sym.FQN
	}
	return sym.Name
}
